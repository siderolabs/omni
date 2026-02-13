// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"iter"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/etcdbackup"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/secrets"
)

type mockStoreFactory struct {
	store etcdbackup.Store
}

func (m *mockStoreFactory) SetThroughputs(uint64, uint64) {}

func (m *mockStoreFactory) GetStore() (etcdbackup.Store, error) {
	return m.store, nil
}

func (m *mockStoreFactory) Start(context.Context, state.State, *zap.Logger) error { return nil }

func (m *mockStoreFactory) Description() string { return "mock-store" }

type mockEtcdBackupStore struct {
	logger      *zap.Logger
	descs       []etcdbackup.Description
	backupDatas []etcdbackup.BackupData
	backups     []etcdbackup.Info
	listCalls   []string
	mu          sync.Mutex
}

func (m *mockEtcdBackupStore) getListCalls() []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.listCalls
}

func (m *mockEtcdBackupStore) getBackups() []etcdbackup.Info {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.backups
}

func (m *mockEtcdBackupStore) ListBackups(_ context.Context, clusterUUID string) (iter.Seq2[etcdbackup.Info, error], error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.logger != nil {
		m.logger.Info("mock list backups", zap.String("uuid", clusterUUID), zap.Any("calls", m.listCalls))
	}

	m.listCalls = append(m.listCalls, clusterUUID)

	return func(yield func(etcdbackup.Info, error) bool) {
		for i, b := range m.backups {
			desc := m.descs[i]
			if desc.ClusterUUID != clusterUUID {
				continue
			}

			if !yield(b, nil) {
				return
			}
		}
	}, nil
}

func (m *mockEtcdBackupStore) Upload(ctx context.Context, desc etcdbackup.Description, rdr io.Reader) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.logger != nil {
		m.logger.Info("mock backup upload", zap.Any("desc", desc))
	}

	backupData := etcdbackup.BackupData{
		AESCBCEncryptionSecret:    desc.EncryptionData.AESCBCEncryptionSecret,
		SecretboxEncryptionSecret: desc.EncryptionData.SecretboxEncryptionSecret,
	}

	data, err := io.ReadAll(rdr)
	if err != nil {
		return err
	}

	m.descs = append(m.descs, desc)
	m.backupDatas = append(m.backupDatas, backupData)
	m.backups = append(m.backups, etcdbackup.Info{
		Timestamp: desc.Timestamp,
		Reader: func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(data)), nil
		},
		Snapshot: etcdbackup.CreateSnapshotName(desc.Timestamp),
		Size:     int64(len(data)),
	})

	return nil
}

func (m *mockEtcdBackupStore) Download(ctx context.Context, _ []byte, clusterUUID, snapshotName string) (etcdbackup.BackupData, io.ReadCloser, error) {
	if ctx.Err() != nil {
		return etcdbackup.BackupData{}, nil, ctx.Err()
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	idx := -1

	for i, desc := range m.descs {
		if desc.ClusterUUID == clusterUUID {
			backup := m.backups[i]
			if backup.Snapshot == snapshotName {
				idx = i

				break
			}
		}
	}

	if idx == -1 {
		return etcdbackup.BackupData{}, nil, fmt.Errorf("not found: %s/%s", clusterUUID, snapshotName)
	}

	backupData := m.backupDatas[idx]
	backup := m.backups[idx]

	rdr, err := backup.Reader()
	if err != nil {
		return etcdbackup.BackupData{}, nil, err
	}

	return backupData, rdr, nil
}

type ClusterBootstrapStatusSuite struct {
	OmniSuite
}

func (suite *ClusterBootstrapStatusSuite) TestReconcile() {
	suite.startRuntime()

	etcdBackupDataMock := etcdbackup.BackupData{
		AESCBCEncryptionSecret:    "test-aes-secret",
		SecretboxEncryptionSecret: "test-secretbox-secret",
	}
	store := &mockEtcdBackupStore{}
	storeFactory := &mockStoreFactory{store: store}

	suite.Require().NoError(suite.runtime.RegisterQController(secrets.NewSecretsController(storeFactory)))
	suite.Require().NoError(suite.runtime.RegisterQController(secrets.NewTalosConfigController(constants.CertificateValidityTime)))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewClusterEndpointController()))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewClusterBootstrapStatusController(storeFactory)))

	clusterName := "talos-default-5"

	cmIdentity := omni.NewClusterMachineIdentity("test-endpoint")
	cmIdentity.TypedSpec().Value.NodeIps = []string{"127.0.0.1"}

	cmIdentity.Metadata().Labels().Set(omni.LabelCluster, clusterName)
	cmIdentity.Metadata().Labels().Set(omni.LabelControlPlaneRole, "")

	suite.Require().NoError(suite.state.Create(suite.ctx, cmIdentity))

	cluster, machines := suite.createCluster(clusterName, 1, 1)

	clusterUUID := "842b441b-abc3-43df-b7c0-51f369eb4fb5"
	clusterUUIDRes := omni.NewClusterUUID(clusterName)
	clusterUUIDRes.TypedSpec().Value.Uuid = clusterUUID

	clusterUUIDRes.Metadata().Labels().Set(omni.LabelClusterUUID, clusterUUID)

	suite.Require().NoError(suite.state.Create(suite.ctx, clusterUUIDRes))

	backupTimestamp := time.Now()

	err := store.Upload(suite.ctx, etcdbackup.Description{
		Timestamp:   backupTimestamp,
		ClusterUUID: clusterUUID,
		ClusterName: cluster.Metadata().ID(),
		EncryptionData: etcdbackup.EncryptionData{
			AESCBCEncryptionSecret:    etcdBackupDataMock.AESCBCEncryptionSecret,
			SecretboxEncryptionSecret: etcdBackupDataMock.SecretboxEncryptionSecret,
		},
	}, strings.NewReader("data"))
	suite.Require().NoError(err)

	for i, m := range machines {
		clusterMachineStatus := omni.NewClusterMachineStatus(m.Metadata().ID())

		clusterMachineStatus.Metadata().Labels().Set(omni.LabelCluster, clusterName)

		if i == 0 {
			clusterMachineStatus.Metadata().Labels().Set(omni.LabelControlPlaneRole, clusterName)
		}

		clusterMachineStatus.TypedSpec().Value.ManagementAddress = suite.socketConnectionString
		clusterMachineStatus.TypedSpec().Value.ApidAvailable = true

		suite.Require().NoError(suite.state.Create(suite.ctx, clusterMachineStatus))
	}

	clusterStatus := omni.NewClusterStatus(cluster.Metadata().ID())
	clusterStatus.TypedSpec().Value.Available = true
	clusterStatus.TypedSpec().Value.HasConnectedControlPlanes = true
	suite.Require().NoError(suite.state.Create(suite.ctx, clusterStatus))

	md := *omni.NewClusterBootstrapStatus(cluster.Metadata().ID()).Metadata()
	assertResource(
		&suite.OmniSuite,
		md,
		func(v *omni.ClusterBootstrapStatus, assertions *assert.Assertions) {
			assertions.True(v.TypedSpec().Value.Bootstrapped, "the cluster is not bootstrapped yet")
		},
	)

	suite.Require().Len(suite.machineService.getBootstrapRequests(), 1)

	suite.testRecoverControlPlaneFromEtcdBackup(cluster.Metadata().ID(), clusterUUID, etcdBackupDataMock, backupTimestamp)

	suite.destroyCluster(cluster)
	rtestutils.Destroy[*omni.ClusterStatus](suite.ctx, suite.T(), suite.state, []resource.ID{clusterStatus.Metadata().ID()})

	suite.Assert().NoError(retry.Constant(5*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		suite.assertNoResource(md),
	))
}

func (suite *ClusterBootstrapStatusSuite) testRecoverControlPlaneFromEtcdBackup(clusterID resource.ID, clusterUUID string, backupDataMock etcdbackup.BackupData, backupTimestamp time.Time) {
	cpMachineSetMd := omni.NewMachineSet(omni.ControlPlanesResourceID(clusterID)).Metadata()

	cpMachineSet, err := safe.StateGet[*omni.MachineSet](suite.ctx, suite.state, cpMachineSetMd)
	suite.Require().NoError(err)

	// destroy the control plane machine set and expect the bootstrapped flag to be cleared
	rtestutils.Destroy[*omni.MachineSet](suite.ctx, suite.T(), suite.state, []resource.ID{cpMachineSetMd.ID()})
	assertResource(
		&suite.OmniSuite,
		omni.NewClusterBootstrapStatus(clusterID).Metadata(),
		func(v *omni.ClusterBootstrapStatus, assertions *assert.Assertions) {
			assertions.False(v.TypedSpec().Value.Bootstrapped, "the cluster still appears to be bootstrapped")
		},
	)

	backupData := omni.NewBackupData(clusterID)
	backupData.TypedSpec().Value.EncryptionKey = []byte("test-key")
	backupData.TypedSpec().Value.AesCbcEncryptionSecret = backupDataMock.AESCBCEncryptionSecret
	backupData.TypedSpec().Value.SecretboxEncryptionSecret = backupDataMock.SecretboxEncryptionSecret

	suite.Require().NoError(suite.state.Create(suite.ctx, backupData))

	// re-create the control plane machine set but now with a bootstrap spec
	cpMachineSet.TypedSpec().Value.BootstrapSpec = &specs.MachineSetSpec_BootstrapSpec{
		ClusterUuid: clusterUUID,
		Snapshot:    etcdbackup.CreateSnapshotName(backupTimestamp),
	}

	suite.Require().NoError(suite.state.Create(suite.ctx, cpMachineSet))

	suite.EventuallyWithT(func(collect *assert.CollectT) {
		assert.Equal(collect, uint64(1), suite.machineService.etcdRecoverRequestCount.Load(), "etcd recover request count is not 1")
		assert.Len(collect, suite.machineService.getBootstrapRequests(), 2, "expected 2 bootstrap requests")
	}, 5*time.Second, 100*time.Millisecond)

	rtestutils.Destroy[*omni.BackupData](suite.ctx, suite.T(), suite.state, []string{backupData.Metadata().ID()})
}

func TestClusterBootstrapStatusSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(ClusterBootstrapStatusSuite))
}
