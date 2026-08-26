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
	"github.com/cosi-project/runtime/pkg/state"
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

// TestBootstrap checks the bootstrap flow: a cluster with a connected control plane is bootstrapped, the
// bootstrapped state is dropped when the control plane machine set goes away, and a new control plane with a
// bootstrap spec gets its etcd restored from the backup.
func TestBootstrap(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()

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

		clusterName := createCluster(ctx, t, tc.State, ms.SocketConnectionString)

		rtestutils.AssertResources(ctx, t, tc.State, []string{clusterName}, func(r *omni.ClusterBootstrapStatus, assertion *assert.Assertions) {
			assertion.True(r.TypedSpec().Value.Bootstrapped)
		})

		require.Len(t, ms.GetBootstrapRequests(), 1)

		// the cluster goes away while its control plane machine set is still there: the finalizer is released
		rtestutils.Destroy[*omni.ClusterStatus](ctx, t, tc.State, []string{clusterName})
		rtestutils.AssertNoResource[*omni.ClusterBootstrapStatus](ctx, t, tc.State, clusterName)

		rtestutils.AssertResources(ctx, t, tc.State, []string{omni.ControlPlanesResourceID(clusterName)}, func(r *omni.MachineSet, assertion *assert.Assertions) {
			assertion.False(r.Metadata().Finalizers().Has(cluster.BootstrapStatusControllerName))
		})
	})
}

// TestRestore checks that a control plane machine set with a bootstrap spec restores etcd from the backup before
// bootstrapping.
func TestRestore(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()

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

		clusterName := "test-restore"

		clusterUUID := omni.NewClusterUUID(clusterName)
		clusterUUID.TypedSpec().Value.Uuid = testClusterUUID
		clusterUUID.Metadata().Labels().Set(omni.LabelClusterUUID, testClusterUUID)
		require.NoError(t, tc.State.Create(ctx, clusterUUID))

		backupData := omni.NewBackupData(clusterName)
		backupData.TypedSpec().Value.EncryptionKey = []byte("test-key")
		backupData.TypedSpec().Value.AesCbcEncryptionSecret = testBackupData.AESCBCEncryptionSecret
		backupData.TypedSpec().Value.SecretboxEncryptionSecret = testBackupData.SecretboxEncryptionSecret
		require.NoError(t, tc.State.Create(ctx, backupData))

		cpMachineSet := omni.NewMachineSet(omni.ControlPlanesResourceID(clusterName))
		cpMachineSet.Metadata().Labels().Set(omni.LabelCluster, clusterName)
		cpMachineSet.Metadata().Labels().Set(omni.LabelControlPlaneRole, "")
		cpMachineSet.TypedSpec().Value.BootstrapSpec = &specs.MachineSetSpec_BootstrapSpec{
			ClusterUuid: testClusterUUID,
			Snapshot:    testSnapshot,
		}
		require.NoError(t, tc.State.Create(ctx, cpMachineSet))

		require.NoError(t, tc.State.Create(ctx, omni.NewTalosConfig(clusterName)))

		clusterEndpoint := omni.NewClusterEndpoint(clusterName)
		clusterEndpoint.TypedSpec().Value.ManagementAddresses = []string{ms.SocketConnectionString}
		require.NoError(t, tc.State.Create(ctx, clusterEndpoint))

		clusterStatus := omni.NewClusterStatus(clusterName)
		clusterStatus.TypedSpec().Value.Available = true
		clusterStatus.TypedSpec().Value.HasConnectedControlPlanes = true
		require.NoError(t, tc.State.Create(ctx, clusterStatus))

		rtestutils.AssertResources(ctx, t, tc.State, []string{clusterName}, func(r *omni.ClusterBootstrapStatus, assertion *assert.Assertions) {
			assertion.True(r.TypedSpec().Value.Bootstrapped)
		})

		require.EqualValues(t, 1, ms.GetEtcdRecoverRequestCount())
		require.Len(t, ms.GetBootstrapRequests(), 1)
		require.True(t, ms.GetBootstrapRequests()[0].RecoverEtcd)
	})
}

// createCluster creates the resources the bootstrap controller acts on: a cluster with a single control plane
// machine reachable at the given address. Returns the cluster name.
func createCluster(ctx context.Context, t *testing.T, st state.State, address string) resource.ID {
	clusterName := "test-" + strings.ToLower(strings.ReplaceAll(t.Name(), "/", "-"))

	cpMachineSet := omni.NewMachineSet(omni.ControlPlanesResourceID(clusterName))
	cpMachineSet.Metadata().Labels().Set(omni.LabelCluster, clusterName)
	cpMachineSet.Metadata().Labels().Set(omni.LabelControlPlaneRole, "")

	clusterEndpoint := omni.NewClusterEndpoint(clusterName)
	clusterEndpoint.TypedSpec().Value.ManagementAddresses = []string{address}

	clusterStatus := omni.NewClusterStatus(clusterName)
	clusterStatus.TypedSpec().Value.Available = true
	clusterStatus.TypedSpec().Value.HasConnectedControlPlanes = true

	for _, res := range []resource.Resource{
		cpMachineSet,
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
