// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package cluster_test

import (
	"context"
	"io"
	"iter"
	"strings"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/cluster"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/etcdbackup"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils"
)

const (
	testClusterUUID = "842b441b-abc3-43df-b7c0-51f369eb4fb5"
	testSnapshot    = "test.snapshot"
)

var testBackupData = etcdbackup.BackupData{
	AESCBCEncryptionSecret:    "test-aes-secret",
	SecretboxEncryptionSecret: "test-secretbox-secret",
}

// TestBootstrap checks the normal flow: the cluster is marked as bootstrapped only once etcd is running on the
// node, the bootstrapped state is dropped when the control plane machine set goes away, and a new control plane
// with a bootstrap spec gets its etcd restored from the backup.
func TestBootstrap(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()

	withBootstrapController(ctx, t, func(ctx context.Context, tc testutils.TestContext, ms *testutils.MachineServiceMock) {
		ms.SetEtcdRunning(false)

		// an accepted bootstrap request brings etcd up some time later, never right away
		ms.SetBootstrapHandler(func(context.Context, *machine.BootstrapRequest) (*machine.BootstrapResponse, error) {
			time.AfterFunc(500*time.Millisecond, func() { ms.SetEtcdRunning(true) })

			return &machine.BootstrapResponse{}, nil
		})

		clusterName := createCluster(ctx, t, tc.State, ms.SocketConnectionString)

		rtestutils.AssertResources(ctx, t, tc.State, []string{clusterName}, func(r *omni.ClusterBootstrapStatus, assertion *assert.Assertions) {
			assertion.True(r.TypedSpec().Value.Bootstrapped)
		})

		require.Len(t, ms.GetBootstrapRequests(), 1)

		// the control plane machine set goes away: the cluster is not bootstrapped anymore, the machine is wiped
		cpMachineSetID := omni.ControlPlanesResourceID(clusterName)

		cpMachineSet, err := safe.StateGetByID[*omni.MachineSet](ctx, tc.State, cpMachineSetID)
		require.NoError(t, err)

		rtestutils.Destroy[*omni.MachineSet](ctx, t, tc.State, []string{cpMachineSetID})
		ms.SetEtcdRunning(false)

		rtestutils.AssertResources(ctx, t, tc.State, []string{clusterName}, func(r *omni.ClusterBootstrapStatus, assertion *assert.Assertions) {
			assertion.False(r.TypedSpec().Value.Bootstrapped)
			assertion.Nil(r.TypedSpec().Value.LastBootstrapAttempt)
		})

		// a new control plane machine set with a bootstrap spec restores etcd from the backup
		backupData := omni.NewBackupData(clusterName)
		backupData.TypedSpec().Value.EncryptionKey = []byte("test-key")
		backupData.TypedSpec().Value.AesCbcEncryptionSecret = testBackupData.AESCBCEncryptionSecret
		backupData.TypedSpec().Value.SecretboxEncryptionSecret = testBackupData.SecretboxEncryptionSecret

		require.NoError(t, tc.State.Create(ctx, backupData))

		cpMachineSet.TypedSpec().Value.BootstrapSpec = &specs.MachineSetSpec_BootstrapSpec{
			ClusterUuid: testClusterUUID,
			Snapshot:    testSnapshot,
		}

		require.NoError(t, tc.State.Create(ctx, cpMachineSet))

		rtestutils.AssertResources(ctx, t, tc.State, []string{clusterName}, func(r *omni.ClusterBootstrapStatus, assertion *assert.Assertions) {
			assertion.True(r.TypedSpec().Value.Bootstrapped)
		})

		// the snapshot is uploaded and the bootstrap request is sent once, nothing is re-sent while etcd is coming up
		require.EqualValues(t, 1, ms.GetEtcdRecoverRequestCount())
		require.Len(t, ms.GetBootstrapRequests(), 2)
		require.True(t, ms.GetBootstrapRequests()[1].RecoverEtcd)

		// the cluster goes away while its control plane machine set is still there: the finalizer is released
		rtestutils.Destroy[*omni.ClusterStatus](ctx, t, tc.State, []string{clusterName})
		rtestutils.AssertNoResource[*omni.ClusterBootstrapStatus](ctx, t, tc.State, clusterName)

		rtestutils.AssertResources(ctx, t, tc.State, []string{cpMachineSetID}, func(r *omni.MachineSet, assertion *assert.Assertions) {
			assertion.False(r.Metadata().Finalizers().Has(cluster.BootstrapStatusControllerName))
		})
	})
}

// TestBootstrapConfirmedByEtcd checks that the cluster is not marked as bootstrapped on the bootstrap request result
// alone, but only once etcd is confirmed running on the node. This is what a node looks like after it rebooted before
// the bootstrap was done and lost it.
func TestBootstrapConfirmedByEtcd(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()

	withBootstrapController(ctx, t, func(ctx context.Context, tc testutils.TestContext, ms *testutils.MachineServiceMock) {
		ms.SetEtcdRunning(false)

		ms.SetBootstrapHandler(func(context.Context, *machine.BootstrapRequest) (*machine.BootstrapResponse, error) {
			return &machine.BootstrapResponse{}, nil
		})

		clusterName := createCluster(ctx, t, tc.State, ms.SocketConnectionString)

		// the request was sent, but the cluster is not marked as bootstrapped on its result alone while etcd is down.
		// the attempt time being recorded on the output resource means the reconcile ran to completion.
		rtestutils.AssertResources(ctx, t, tc.State, []string{clusterName}, func(r *omni.ClusterBootstrapStatus, assertion *assert.Assertions) {
			assertion.NotNil(r.TypedSpec().Value.LastBootstrapAttempt)
			assertion.False(r.TypedSpec().Value.Bootstrapped, "the cluster must not be marked as bootstrapped before etcd is running")
		})

		ms.SetEtcdRunning(true)

		rtestutils.AssertResources(ctx, t, tc.State, []string{clusterName}, func(r *omni.ClusterBootstrapStatus, assertion *assert.Assertions) {
			assertion.True(r.TypedSpec().Value.Bootstrapped, "the cluster must be marked as bootstrapped once etcd is running")
		})

		// a single request was sent across the whole flow: it was not re-sent while etcd was being waited for
		require.Len(t, ms.GetBootstrapRequests(), 1, "the bootstrap request must not be re-sent while etcd is being waited for")
	})
}

// TestBootstrapAlreadyExists checks that a node which already has etcd data does not get the controller stuck: the
// cluster is marked as bootstrapped once etcd is running, without another bootstrap request having to succeed.
func TestBootstrapAlreadyExists(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()

	withBootstrapController(ctx, t, func(ctx context.Context, tc testutils.TestContext, ms *testutils.MachineServiceMock) {
		ms.SetEtcdRunning(false)

		ms.SetBootstrapHandler(func(context.Context, *machine.BootstrapRequest) (*machine.BootstrapResponse, error) {
			return nil, status.Error(codes.AlreadyExists, "etcd data directory is not empty")
		})

		clusterName := createCluster(ctx, t, tc.State, ms.SocketConnectionString)

		// the request came back with AlreadyExists while etcd is down: the controller records the attempt and keeps
		// going instead of getting stuck, and does not mark the cluster as bootstrapped yet.
		rtestutils.AssertResources(ctx, t, tc.State, []string{clusterName}, func(r *omni.ClusterBootstrapStatus, assertion *assert.Assertions) {
			assertion.NotNil(r.TypedSpec().Value.LastBootstrapAttempt)
			assertion.False(r.TypedSpec().Value.Bootstrapped)
		})

		ms.SetEtcdRunning(true)

		rtestutils.AssertResources(ctx, t, tc.State, []string{clusterName}, func(r *omni.ClusterBootstrapStatus, assertion *assert.Assertions) {
			assertion.True(r.TypedSpec().Value.Bootstrapped)
		})
	})
}

// TestBootstrapRejectedRetriesSoon checks that a request the node rejected without starting anything is retried at
// the next check, instead of being held back by the retry interval which only protects a request that might still be
// running on the node. A node rejects a bootstrap this way while it is still coming up, which is an ordinary race
// during cluster creation.
func TestBootstrapRejectedRetriesSoon(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()

	withBootstrapController(ctx, t, func(ctx context.Context, tc testutils.TestContext, ms *testutils.MachineServiceMock) {
		ms.SetEtcdRunning(false)

		ms.SetBootstrapHandler(func(context.Context, *machine.BootstrapRequest) (*machine.BootstrapResponse, error) {
			// the node is not ready to be bootstrapped when the first request arrives
			if len(ms.GetBootstrapRequests()) == 1 {
				return nil, status.Error(codes.FailedPrecondition, "bootstrap is not available yet")
			}

			return &machine.BootstrapResponse{}, nil
		})

		clusterName := createCluster(ctx, t, tc.State, ms.SocketConnectionString)

		// a rejected request is not recorded as an attempt, so the attempt time showing up means the retry landed
		rtestutils.AssertResources(ctx, t, tc.State, []string{clusterName}, func(r *omni.ClusterBootstrapStatus, assertion *assert.Assertions) {
			assertion.NotNil(r.TypedSpec().Value.LastBootstrapAttempt)
		})

		require.Len(t, ms.GetBootstrapRequests(), 2, "the rejected request must be retried")

		ms.SetEtcdRunning(true)

		rtestutils.AssertResources(ctx, t, tc.State, []string{clusterName}, func(r *omni.ClusterBootstrapStatus, assertion *assert.Assertions) {
			assertion.True(r.TypedSpec().Value.Bootstrapped)
		})
	})
}

// TestBootstrapErrorSavesAttempt checks that a bootstrap request which fails with an error is still counted as sent,
// as the node might have accepted it, so it is not re-sent right away.
func TestBootstrapErrorSavesAttempt(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()

	withBootstrapController(ctx, t, func(ctx context.Context, tc testutils.TestContext, ms *testutils.MachineServiceMock) {
		ms.SetEtcdRunning(false)

		ms.SetBootstrapHandler(func(context.Context, *machine.BootstrapRequest) (*machine.BootstrapResponse, error) {
			return nil, status.Error(codes.DeadlineExceeded, "context deadline exceeded")
		})

		clusterName := createCluster(ctx, t, tc.State, ms.SocketConnectionString)

		// the request failed, but the attempt time is still recorded so the request is not re-sent right away
		rtestutils.AssertResources(ctx, t, tc.State, []string{clusterName}, func(r *omni.ClusterBootstrapStatus, assertion *assert.Assertions) {
			assertion.NotNil(r.TypedSpec().Value.LastBootstrapAttempt, "the attempt must be saved even though the request failed")
		})
	})
}

// withBootstrapController runs the test with the bootstrap controller registered and a Talos machine mock which has a
// backup for the test cluster.
func withBootstrapController(ctx context.Context, t *testing.T, test func(ctx context.Context, tc testutils.TestContext, ms *testutils.MachineServiceMock)) {
	store := &mockEtcdBackupStore{}

	testutils.WithRuntime(ctx, t, testutils.TestOptions{}, func(_ context.Context, tc testutils.TestContext) {
		require.NoError(t, tc.Runtime.RegisterQController(cluster.NewBootstrapStatusController(&mockStoreFactory{store: store})))
	}, func(ctx context.Context, tc testutils.TestContext) {
		ms := testutils.NewMachineServiceMock(ctx, t, "", tc.State)

		require.NoError(t, store.Upload(ctx, etcdbackup.Description{
			ClusterUUID: testClusterUUID,
			EncryptionData: etcdbackup.EncryptionData{
				AESCBCEncryptionSecret:    testBackupData.AESCBCEncryptionSecret,
				SecretboxEncryptionSecret: testBackupData.SecretboxEncryptionSecret,
			},
		}, strings.NewReader("data")))

		test(ctx, tc, ms)
	})
}

// createCluster creates the resources the bootstrap controller acts on: a cluster with a single control plane
// machine reachable at the given address. Returns the cluster name.
func createCluster(ctx context.Context, t *testing.T, st state.State, address string) resource.ID {
	clusterName := "test-" + strings.ToLower(strings.ReplaceAll(t.Name(), "/", "-"))

	cpMachineSet := omni.NewMachineSet(omni.ControlPlanesResourceID(clusterName))
	cpMachineSet.Metadata().Labels().Set(omni.LabelCluster, clusterName)
	cpMachineSet.Metadata().Labels().Set(omni.LabelControlPlaneRole, "")

	clusterUUID := omni.NewClusterUUID(clusterName)
	clusterUUID.TypedSpec().Value.Uuid = testClusterUUID
	clusterUUID.Metadata().Labels().Set(omni.LabelClusterUUID, testClusterUUID)

	clusterEndpoint := omni.NewClusterEndpoint(clusterName)
	clusterEndpoint.TypedSpec().Value.ManagementAddresses = []string{address}

	clusterStatus := omni.NewClusterStatus(clusterName)
	clusterStatus.TypedSpec().Value.Available = true
	clusterStatus.TypedSpec().Value.HasConnectedControlPlanes = true

	for _, res := range []resource.Resource{
		cpMachineSet,
		clusterUUID,
		clusterEndpoint,
		omni.NewTalosConfig(clusterName),
		clusterStatus,
	} {
		require.NoError(t, st.Create(ctx, res))
	}

	return clusterName
}

type mockStoreFactory struct {
	store etcdbackup.Store
}

func (m *mockStoreFactory) SetThroughputs(uint64, uint64) {}

func (m *mockStoreFactory) GetStore() (etcdbackup.Store, error) { return m.store, nil }

func (m *mockStoreFactory) Start(context.Context, state.State, *zap.Logger) error { return nil }

func (m *mockStoreFactory) Description() string { return "mock-store" }

// mockEtcdBackupStore holds a single backup.
type mockEtcdBackupStore struct {
	data        string
	description etcdbackup.Description
}

func (m *mockEtcdBackupStore) ListBackups(context.Context, string) (iter.Seq2[etcdbackup.Info, error], error) {
	return func(func(etcdbackup.Info, error) bool) {}, nil
}

func (m *mockEtcdBackupStore) Upload(_ context.Context, desc etcdbackup.Description, rdr io.Reader) error {
	data, err := io.ReadAll(rdr)
	if err != nil {
		return err
	}

	m.description = desc
	m.data = string(data)

	return nil
}

func (m *mockEtcdBackupStore) Download(_ context.Context, _ []byte, clusterUUID, snapshotName string) (etcdbackup.BackupData, io.ReadCloser, error) {
	if clusterUUID != m.description.ClusterUUID || snapshotName != testSnapshot {
		return etcdbackup.BackupData{}, nil, status.Errorf(codes.NotFound, "not found: %s/%s", clusterUUID, snapshotName)
	}

	return etcdbackup.BackupData{
		AESCBCEncryptionSecret:    m.description.EncryptionData.AESCBCEncryptionSecret,
		SecretboxEncryptionSecret: m.description.EncryptionData.SecretboxEncryptionSecret,
	}, io.NopCloser(strings.NewReader(m.data)), nil
}
