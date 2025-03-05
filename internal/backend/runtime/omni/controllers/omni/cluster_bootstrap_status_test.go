// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

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
	"github.com/siderolabs/gen/xiter"
	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/etcdbackup"
)

type mockStoreFactory struct {
	etcdBackupDataMock etcdbackup.BackupData
}

func (m *mockStoreFactory) GetStore() (etcdbackup.Store, error) {
	return &mockEtcdBackupStore{m.etcdBackupDataMock}, nil
}

func (m *mockStoreFactory) Start(context.Context, state.State, *zap.Logger) error { return nil }

func (m *mockStoreFactory) Description() string { return "mock-store" }

type mockEtcdBackupStore struct {
	etcdBackupDataMock etcdbackup.BackupData
}

func (m *mockEtcdBackupStore) ListBackups(context.Context, string) (iter.Seq2[etcdbackup.Info, error], error) {
	return xiter.Empty2, nil
}

func (m *mockEtcdBackupStore) Upload(context.Context, etcdbackup.Description, io.Reader) error {
	return nil
}

func (m *mockEtcdBackupStore) Download(context.Context, []byte, string, string) (etcdbackup.BackupData, io.ReadCloser, error) {
	return m.etcdBackupDataMock, io.NopCloser(strings.NewReader("test-data")), nil
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

	storeFactory := &mockStoreFactory{etcdBackupDataMock}

	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewSecretsController(storeFactory)))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewTalosConfigController(constants.CertificateValidityTime)))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewClusterEndpointController()))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewClusterBootstrapStatusController(storeFactory)))

	clusterName := "talos-default-5"

	cmIdentity := omni.NewClusterMachineIdentity(resources.DefaultNamespace, "test-endpoint")
	cmIdentity.TypedSpec().Value.NodeIps = []string{"127.0.0.1"}

	cmIdentity.Metadata().Labels().Set(omni.LabelCluster, clusterName)
	cmIdentity.Metadata().Labels().Set(omni.LabelControlPlaneRole, "")

	suite.Require().NoError(suite.state.Create(suite.ctx, cmIdentity))

	cluster, machines := suite.createCluster(clusterName, 1, 1)

	for i, m := range machines {
		clusterMachineStatus := omni.NewClusterMachineStatus(resources.DefaultNamespace, m.Metadata().ID())

		clusterMachineStatus.Metadata().Labels().Set(omni.LabelCluster, clusterName)

		if i == 0 {
			clusterMachineStatus.Metadata().Labels().Set(omni.LabelControlPlaneRole, clusterName)
		}

		clusterMachineStatus.TypedSpec().Value.ManagementAddress = suite.socketConnectionString
		clusterMachineStatus.TypedSpec().Value.ApidAvailable = true

		suite.Require().NoError(suite.state.Create(suite.ctx, clusterMachineStatus))
	}

	clusterStatus := omni.NewClusterStatus(resources.DefaultNamespace, cluster.Metadata().ID())
	clusterStatus.TypedSpec().Value.Available = true
	clusterStatus.TypedSpec().Value.HasConnectedControlPlanes = true
	suite.Require().NoError(suite.state.Create(suite.ctx, clusterStatus))

	md := *omni.NewClusterBootstrapStatus(resources.DefaultNamespace, cluster.Metadata().ID()).Metadata()
	assertResource(
		&suite.OmniSuite,
		md,
		func(v *omni.ClusterBootstrapStatus, assertions *assert.Assertions) {
			assertions.True(v.TypedSpec().Value.Bootstrapped, "the cluster is not bootstrapped yet")
		},
	)

	suite.Require().Len(suite.machineService.getBootstrapRequests(), 1)

	suite.testRecoverControlPlaneFromEtcdBackup(cluster.Metadata().ID(), etcdBackupDataMock)

	suite.destroyCluster(cluster)
	rtestutils.Destroy[*omni.ClusterStatus](suite.ctx, suite.T(), suite.state, []resource.ID{clusterStatus.Metadata().ID()})

	suite.Assert().NoError(retry.Constant(5*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		suite.assertNoResource(md),
	))
}

func (suite *ClusterBootstrapStatusSuite) testRecoverControlPlaneFromEtcdBackup(clusterID resource.ID, backupDataMock etcdbackup.BackupData) {
	cpMachineSetMd := omni.NewMachineSet(resources.DefaultNamespace, omni.ControlPlanesResourceID(clusterID)).Metadata()

	cpMachineSet, err := safe.StateGet[*omni.MachineSet](suite.ctx, suite.state, cpMachineSetMd)
	suite.Require().NoError(err)

	// destroy the control plane machine set and expect the bootstrapped flag to be cleared
	rtestutils.Destroy[*omni.MachineSet](suite.ctx, suite.T(), suite.state, []resource.ID{cpMachineSetMd.ID()})
	assertResource(
		&suite.OmniSuite,
		omni.NewClusterBootstrapStatus(resources.DefaultNamespace, clusterID).Metadata(),
		func(v *omni.ClusterBootstrapStatus, assertions *assert.Assertions) {
			assertions.False(v.TypedSpec().Value.Bootstrapped, "the cluster still appears to be bootstrapped")
		},
	)

	backupData := omni.NewBackupData(clusterID)
	backupData.TypedSpec().Value.EncryptionKey = []byte("test-key")
	backupData.TypedSpec().Value.AesCbcEncryptionSecret = backupDataMock.AESCBCEncryptionSecret
	backupData.TypedSpec().Value.SecretboxEncryptionSecret = backupDataMock.SecretboxEncryptionSecret

	suite.Require().NoError(suite.state.Create(suite.ctx, backupData))

	clusterUUID := "842b441b-abc3-43df-b7c0-51f369eb4fb5"
	clusterUUIDRes := omni.NewClusterUUID(clusterID)
	clusterUUIDRes.TypedSpec().Value.Uuid = clusterUUID

	clusterUUIDRes.Metadata().Labels().Set(omni.LabelClusterUUID, clusterUUID)

	suite.Require().NoError(suite.state.Create(suite.ctx, clusterUUIDRes))

	// re-create the control plane machine set but now with a bootstrap spec
	cpMachineSet.TypedSpec().Value.BootstrapSpec = &specs.MachineSetSpec_BootstrapSpec{
		ClusterUuid: clusterUUID,
		Snapshot:    etcdbackup.CreateSnapshotName(time.Now()),
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
