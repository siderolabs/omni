// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"context"
	"errors"
	"io"
	"iter"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"
	"github.com/jonboulle/clockwork"
	"github.com/siderolabs/gen/containers"
	"github.com/siderolabs/gen/xiter"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/gen/xtesting/must"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/etcdbackup"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/etcdbackup/store"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/etcdbackup/crypt"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/external"
	"github.com/siderolabs/omni/internal/pkg/xmocks"
)

func TestEtcdBackupControllerSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(EtcdBackupControllerSuite))
}

type EtcdBackupControllerSuite struct {
	OmniSuite
}

func (suite *EtcdBackupControllerSuite) register(ctrl controller.Controller) {
	suite.Require().NoError(suite.runtime.RegisterController(ctrl))
}

func (suite *EtcdBackupControllerSuite) qregister(ctrl controller.QController) {
	suite.Require().NoError(suite.runtime.RegisterQController(ctrl))
}

func (suite *EtcdBackupControllerSuite) register2(ctrl controller.Controller, err error) {
	suite.Require().NoError(err)
	suite.register(ctrl)
}

func (suite *EtcdBackupControllerSuite) SetupTest() {
	suite.OmniSuite.SetupTest()

	suite.startRuntime()

	suite.register(omnictrl.NewClusterController())
	suite.qregister(omnictrl.NewClusterUUIDController())
	suite.qregister(omnictrl.NewEtcdBackupEncryptionController())
	suite.qregister(omnictrl.NewSecretsController(suite.fileStoreFactory()))
	suite.qregister(omnictrl.NewBackupDataController())
}

func (suite *EtcdBackupControllerSuite) TestEtcdBackup() {
	fakeclock := fakeClock()
	sf := suite.fileStoreFactory()
	clientMock := &talosClientMock{}

	defer clientMock.AssertExpectations(suite.T())

	clientMock.
		On(xmocks.Name((*talosClientMock).EtcdSnapshot), mock.Anything, mock.Anything, mock.Anything).
		Return(func() (io.ReadCloser, error) { return io.NopCloser(strings.NewReader("Hello World")), nil })

	suite.register2(omnictrl.NewEtcdBackupController(omnictrl.EtcdBackupControllerSettings{
		ClientMaker: func(context.Context, string) (omnictrl.TalosClient, error) {
			return clientMock, nil
		},
		StoreFactory: sf,
		Clock:        fakeclock,
		TickInterval: 10 * time.Minute,
	}))

	clusterNames := []string{"talos-default-1", "talos-default-2"}
	clustersData := createClusters(suite, clusterNames, time.Hour)
	start := fakeclock.Now()

	blockAndAdvance(suite.ctx, suite.T(), fakeclock, 11*time.Minute)

	rtestutils.AssertResources(
		suite.ctx,
		suite.T(),
		suite.state,
		xslices.Map(clustersData, func(cluster *omni.BackupData) resource.ID { return cluster.Metadata().ID() }),
		func(r *omni.EtcdBackupStatus, assertion *assert.Assertions) {
			ids := xslices.Map(clustersData, func(cluster *omni.BackupData) resource.ID { return cluster.Metadata().ID() })

			if !slices.Contains(ids, r.Metadata().ID()) {
				assertion.Failf("unexpected cluster %s", r.Metadata().ID())
			}

			value := r.TypedSpec().Value

			assertion.Equal(specs.EtcdBackupStatusSpec_Ok, value.Status)
			assertion.Zero(value.Error)
			assertion.WithinRange(value.LastBackupTime.AsTime(), start, fakeclock.Now())
		},
	)

	// Backups should be created for both clusters since those backups do not exist yet
	for _, cd := range clustersData {
		suite.eventuallyFindBackups(sf, cd.Metadata().ID(), 1)
	}

	blockAndAdvance(suite.ctx, suite.T(), fakeclock, time.Hour)

	for _, cd := range clustersData {
		suite.eventuallyFindBackups(sf, cd.Metadata().ID(), 2)
	}

	// Destroy the first cluster
	suite.destroyClusterByID(clustersData[0].Metadata().ID())

	blockAndAdvance(suite.ctx, suite.T(), fakeclock, time.Hour)

	suite.eventuallyFindBackups(sf, clustersData[1].Metadata().ID(), 3)

	// Destroy the second cluster
	suite.destroyClusterByID(clustersData[1].Metadata().ID())
}

func (suite *EtcdBackupControllerSuite) TestEtcdBackupFactoryFails() {
	fakeclock := fakeClock()
	sf := suite.fileStoreFactory()
	clientMock := &talosClientMock{}

	defer clientMock.AssertExpectations(suite.T())

	clientMock.
		On(xmocks.Name((*talosClientMock).EtcdSnapshot), mock.Anything, mock.Anything, mock.Anything).
		Return(func() (io.ReadCloser, error) { return io.NopCloser(strings.NewReader("Hello World")), nil }).
		Times(5)

	var m containers.ConcurrentMap[string, func() (omnictrl.TalosClient, error)]

	clusterNames := []string{"talos-default-3", "talos-default-4", "talos-default-5"}

	for _, clusterName := range clusterNames {
		m.Set(clusterName, func() (omnictrl.TalosClient, error) {
			return clientMock, nil
		})
	}

	suite.register2(omnictrl.NewEtcdBackupController(omnictrl.EtcdBackupControllerSettings{
		ClientMaker: func(_ context.Context, clusterName string) (omnictrl.TalosClient, error) {
			fn, _ := m.Get(clusterName)

			return fn()
		},
		StoreFactory: sf,
		Clock:        fakeclock,
		TickInterval: 10 * time.Minute,
	}))

	clustersData := createClusters(suite, clusterNames, time.Hour)

	blockAndAdvance(suite.ctx, suite.T(), fakeclock, 11*time.Minute)

	// Backups should be created for both clusters since those backups do not exist yet
	for _, cluster := range clustersData {
		suite.eventuallyFindBackups(sf, cluster.Metadata().ID(), 1)
	}

	start := fakeclock.Now()

	m.Set(clusterNames[0], func() (omnictrl.TalosClient, error) {
		return nil, errors.New("failed to create client")
	})

	blockAndAdvance(suite.ctx, suite.T(), fakeclock, time.Hour)

	assertResource(
		&suite.OmniSuite,
		clustersData[0].Metadata(),
		func(r *omni.EtcdBackupStatus, assertion *assert.Assertions) {
			value := r.TypedSpec().Value

			assertion.EqualValuesf(value.Error, "failed to create talos client for cluster, skipping cluster backup: failed to create client", "cluster %s", r.Metadata().ID())
			assertion.Equalf(specs.EtcdBackupStatusSpec_Error, value.Status, "cluster %s", r.Metadata().ID())
			assertion.WithinRangef(
				value.LastBackupAttempt.AsTime(),
				start,
				fakeclock.Now(),
				"cluster %s",
				r.Metadata().ID(),
			)
		},
	)

	suite.eventuallyFindBackups(sf, clustersData[0].Metadata().ID(), 1)
	suite.eventuallyFindBackups(sf, clustersData[1].Metadata().ID(), 2)

	for i := range clusterNames {
		suite.destroyClusterByID(clustersData[i].Metadata().ID())
	}
}

func (suite *EtcdBackupControllerSuite) TestDecryptEtcdBackup() {
	fakeclock := fakeClock()
	sf := suite.fileStoreFactory()
	clientMock := &talosClientMock{}

	defer clientMock.AssertExpectations(suite.T())

	clientMock.
		On(xmocks.Name((*talosClientMock).EtcdSnapshot), mock.Anything, mock.Anything, mock.Anything).
		Return(func() (io.ReadCloser, error) { return io.NopCloser(strings.NewReader("Hello World")), nil })

	suite.register2(omnictrl.NewEtcdBackupController(omnictrl.EtcdBackupControllerSettings{
		ClientMaker: func(context.Context, string) (omnictrl.TalosClient, error) {
			return clientMock, nil
		},
		StoreFactory: sf,
		Clock:        fakeclock,
		TickInterval: 10 * time.Minute,
	}))

	clusterNames := []string{"talos-default-6"}
	clusters := createClusters(suite, clusterNames, time.Hour)

	blockAndAdvance(suite.ctx, suite.T(), fakeclock, 11*time.Minute)

	clusterBackups := suite.eventuallyFindBackups(sf, clusters[0].Metadata().ID(), 1)

	src := must.Value(clusterBackups[0].Reader())(suite.T())

	suite.T().Cleanup(func() { suite.Require().NoError(src.Close()) })

	decryptedHeader, decrypter := must.Values(crypt.Decrypt(
		src,
		clusters[0].TypedSpec().Value.EncryptionKey,
	))(suite.T())

	suite.Require().EqualValues(
		clusters[0].TypedSpec().Value.AesCbcEncryptionSecret,
		decryptedHeader.AESCBCEncryptionSecret,
	)
	suite.Require().EqualValues(
		clusters[0].TypedSpec().Value.SecretboxEncryptionSecret,
		decryptedHeader.SecretboxEncryptionSecret,
	)
	suite.Require().EqualValues("Hello World", string(must.Value(io.ReadAll(decrypter))(suite.T())))

	for i := range clusterNames {
		suite.destroyClusterByID(clusters[i].Metadata().ID())
	}
}

func (suite *EtcdBackupControllerSuite) TestSingleListCall() {
	fakeclock := fakeClock()
	sfm := &storeFactoryMock{}
	sm := &storeMock{}

	defer sfm.AssertExpectations(suite.T())
	defer sm.AssertExpectations(suite.T())

	ch := make(chan string, 3)

	sfm.On(xmocks.Name((*sfm).GetStore)).Return(sm, nil)

	sm.On(xmocks.Name((*storeMock).ListBackups), mock.Anything, mock.Anything).Return(iter.Seq2[etcdbackup.Info, error](xiter.Empty2[etcdbackup.Info, error]), nil).Twice()
	sm.On(xmocks.Name((*storeMock).Upload), mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		ch <- xmocks.GetAs[etcdbackup.Description](args, 1).ClusterName
		must.Value(io.Copy(io.Discard, xmocks.GetAs[io.Reader](args, 2)))
	}).Return(nil).Times(3)

	clientMock := &talosClientMock{}
	defer clientMock.AssertExpectations(suite.T())

	clientMock.
		On(xmocks.Name((*talosClientMock).EtcdSnapshot), mock.Anything, mock.Anything, mock.Anything).
		Return(func() (io.ReadCloser, error) { return io.NopCloser(strings.NewReader("Hello World")), nil })

	suite.register2(omnictrl.NewEtcdBackupController(omnictrl.EtcdBackupControllerSettings{
		ClientMaker: func(context.Context, string) (omnictrl.TalosClient, error) {
			return clientMock, nil
		},
		StoreFactory: sfm,
		Clock:        fakeclock,
		TickInterval: 10 * time.Minute,
	}))

	clusterNames := []string{"talos-default-7", "talos-default-8"}
	clusters := createClusters(suite, clusterNames, time.Hour)

	blockAndAdvance(suite.ctx, suite.T(), fakeclock, 11*time.Minute)

	suite.destroyClusterByID(clusters[0].Metadata().ID())

	blockAndAdvance(suite.ctx, suite.T(), fakeclock, time.Hour)

	suite.Require().NoError(fakeclock.BlockUntilContext(suite.ctx, 1))

	for i := range clusterNames {
		suite.destroyClusterByID(clusters[i].Metadata().ID())
	}
}

func (suite *EtcdBackupControllerSuite) TestListBackupsWithExistingData() {
	clusterNames := []string{"talos-default-9", "talos-default-10"}
	clusters := createClusters(suite, clusterNames, time.Hour)

	fakeclock := fakeClock()
	sfm := &storeFactoryMock{}
	store := &storeMock{}

	defer sfm.AssertExpectations(suite.T())
	defer store.AssertExpectations(suite.T())

	sfm.On(xmocks.Name((*sfm).GetStore)).Return(store, nil)

	store.
		On(xmocks.Name((*storeMock).ListBackups), mock.Anything, mock.Anything).
		Return(toIter([]etcdbackup.Info{
			{Timestamp: fakeclock.Now().Add(time.Minute), Reader: nil, Size: 0},
			{Timestamp: fakeclock.Now(), Reader: nil, Size: 0},
		}), nil).
		Twice()
	store.On(xmocks.Name((*storeMock).Upload), mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		must.Value(io.Copy(io.Discard, xmocks.GetAs[io.Reader](args, 2)))
	}).Return(nil).Twice()

	clientMock := &talosClientMock{}
	defer clientMock.AssertExpectations(suite.T())

	clientMock.
		On(xmocks.Name((*talosClientMock).EtcdSnapshot), mock.Anything, mock.Anything, mock.Anything).
		Return(func() (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("Hello World")), nil
		})

	suite.register2(omnictrl.NewEtcdBackupController(omnictrl.EtcdBackupControllerSettings{
		ClientMaker: func(context.Context, string) (omnictrl.TalosClient, error) {
			return clientMock, nil
		},
		StoreFactory: sfm,
		Clock:        fakeclock,
		TickInterval: 10 * time.Minute,
	}))

	blockAndAdvance(suite.ctx, suite.T(), fakeclock, 11*time.Minute)

	suite.destroyClusterByID(clusters[0].Metadata().ID())

	blockAndAdvance(suite.ctx, suite.T(), fakeclock, time.Hour)
	blockAndAdvance(suite.ctx, suite.T(), fakeclock, time.Hour)

	suite.Require().NoError(fakeclock.BlockUntilContext(suite.ctx, 1))

	suite.destroyClusterByID(clusters[1].Metadata().ID())
}

func (suite *EtcdBackupControllerSuite) TestEtcdManualBackupFindResource() {
	fakeclock := fakeClock()
	sf := suite.fileStoreFactory()
	clientMock := &talosClientMock{}

	suite.stateBuilder.Set(resources.ExternalNamespace, &external.State{
		CoreState:    suite.state,
		StoreFactory: sf,
	})

	defer clientMock.AssertExpectations(suite.T())

	clientMock.
		On(xmocks.Name((*talosClientMock).EtcdSnapshot), mock.Anything, mock.Anything, mock.Anything).
		Return(func() (io.ReadCloser, error) { return io.NopCloser(strings.NewReader("Hello World")), nil })

	suite.register2(omnictrl.NewEtcdBackupController(omnictrl.EtcdBackupControllerSettings{
		ClientMaker: func(context.Context, string) (omnictrl.TalosClient, error) {
			return clientMock, nil
		},
		StoreFactory: sf,
		Clock:        fakeclock,
		TickInterval: time.Minute,
	}))

	clusterNames := []string{"talos-default-11"}
	clustersData := createClusters(suite, clusterNames, 0) // 0 means that automatic backups are disabled

	manualBackup := omni.NewEtcdManualBackup(clustersData[0].Metadata().ID())
	manualBackup.TypedSpec().Value.BackupAt = timestamppb.New(fakeclock.Now().Add(15 * time.Second))

	err := suite.state.Create(suite.ctx, manualBackup)
	suite.Require().NoError(err)

	blockAndAdvance(suite.ctx, suite.T(), fakeclock, 15*time.Second)

	backups := suite.eventuallyFindBackups(sf, clustersData[0].Metadata().ID(), 1)

	// Should find backups by cluster ID

	backupRes := must.Value(safe.StateListAll[*omni.EtcdBackup](
		suite.ctx,
		suite.state,
		state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, clusterNames[0])),
	))(suite.T())

	suite.Assert().EqualValues(len(backups), backupRes.Len())

	suite.Assert().Equal(
		omni.NewEtcdBackup(clusterNames[0], backups[0].Timestamp).Metadata().ID(),
		backupRes.Get(0).Metadata().ID(),
	)

	suite.Assert().Equal(backups[0].Snapshot, backupRes.Get(0).TypedSpec().Value.Snapshot)
	suite.Assert().Equal(backups[0].Timestamp.UTC(), backupRes.Get(0).TypedSpec().Value.CreatedAt.AsTime())

	// Should find backup by backup id

	res := must.Value(safe.StateGetByID[*omni.EtcdBackup](suite.ctx, suite.state, backupRes.Get(0).Metadata().ID()))(suite.T())

	suite.Assert().Equal(backups[0].Snapshot, res.TypedSpec().Value.Snapshot)
	suite.Assert().Equal(backups[0].Timestamp.UTC(), res.TypedSpec().Value.CreatedAt.AsTime())

	// Should return an error if no query is provided

	_, err = safe.StateList[*omni.EtcdBackup](suite.ctx, suite.state, omni.NewEtcdBackup("", fakeclock.Now()).Metadata())
	suite.Require().Error(err)

	for _, clusterData := range clustersData {
		suite.destroyClusterByID(clusterData.Metadata().ID())
	}
}

func (suite *EtcdBackupControllerSuite) TestEtcdManualBackup() {
	fakeclock := fakeClock()
	sf := suite.fileStoreFactory()
	clientMock := &talosClientMock{}

	defer clientMock.AssertExpectations(suite.T())

	clientMock.
		On(xmocks.Name((*talosClientMock).EtcdSnapshot), mock.Anything, mock.Anything, mock.Anything).
		Return(func() (io.ReadCloser, error) { return io.NopCloser(strings.NewReader("Hello World")), nil })

	suite.register2(omnictrl.NewEtcdBackupController(omnictrl.EtcdBackupControllerSettings{
		ClientMaker: func(context.Context, string) (omnictrl.TalosClient, error) {
			return clientMock, nil
		},
		StoreFactory: sf,
		Clock:        fakeclock,
		TickInterval: time.Minute,
	}))

	clusterNames := []string{"talos-default-12", "talos-default-13"}
	clustersData := createClusters(suite, clusterNames, 0) // 0 means that automatic backups are disabled

	blockAndAdvance(suite.ctx, suite.T(), fakeclock, 15*time.Second)

	assertNoResource(&suite.OmniSuite, omni.NewEtcdBackupStatus(clustersData[0].Metadata().ID()))
	assertNoResource(&suite.OmniSuite, omni.NewEtcdBackupStatus(clustersData[1].Metadata().ID()))

	manualBackup := omni.NewEtcdManualBackup(clustersData[0].Metadata().ID())
	manualBackup.TypedSpec().Value.BackupAt = timestamppb.New(fakeclock.Now().Add(12 * time.Minute))

	err := suite.state.Create(suite.ctx, manualBackup)
	suite.Require().NoError(err)

	now := fakeclock.Now()

	assertNoResource(&suite.OmniSuite, omni.NewEtcdBackupStatus(clustersData[0].Metadata().ID()))
	suite.eventuallyFindBackups(sf, clustersData[0].Metadata().ID(), 0)

	blockAndAdvance(suite.ctx, suite.T(), fakeclock, 12*time.Minute)

	assertResource(
		&suite.OmniSuite,
		clustersData[0].Metadata(),
		func(r *omni.EtcdBackupStatus, assertion *assert.Assertions) {
			value := r.TypedSpec().Value

			assertion.Zero(value.Error, "cluster %s", r.Metadata().ID())
			assertion.Equalf(specs.EtcdBackupStatusSpec_Ok, value.Status, "cluster %s", r.Metadata().ID())
			assertion.WithinRange(
				value.LastBackupTime.AsTime(),
				now,
				fakeclock.Now(),
				"cluster %s",
				r.Metadata().ID(),
			)
			assertion.WithinRange(
				value.LastBackupAttempt.AsTime(),
				now,
				fakeclock.Now(),
				"cluster %s",
				r.Metadata().ID(),
			)
		},
	)

	suite.eventuallyFindBackups(sf, clustersData[0].Metadata().ID(), 1)

	blockAndAdvance(suite.ctx, suite.T(), fakeclock, 10*time.Minute)

	// Should ignore this backup
	_, err = safe.StateUpdateWithConflicts(suite.ctx, suite.state, manualBackup.Metadata(), func(b *omni.EtcdManualBackup) error {
		b.TypedSpec().Value.BackupAt = timestamppb.New(fakeclock.Now().Add(-12 * time.Minute))

		return nil
	})
	suite.Require().NoError(err)

	suite.eventuallyFindBackups(sf, clustersData[0].Metadata().ID(), 1)

	for _, clusterData := range clustersData {
		suite.destroyClusterByID(clusterData.Metadata().ID())
	}
}

func (suite *EtcdBackupControllerSuite) TestS3Backup() {
	fakeclock := fakeClock()
	logger := zaptest.NewLogger(suite.T())

	backend := newFakeBackend(logger)
	faker := gofakes3.New(backend)

	ts := httptest.NewServer(faker.Server())
	defer ts.Close()

	const bucket = "test-bucket"

	suite.Require().NoError(backend.CreateBucket(bucket))

	conf := omni.NewEtcdBackupS3Conf()
	conf.TypedSpec().Value.Bucket = bucket
	conf.TypedSpec().Value.Region = "us-east-1"
	conf.TypedSpec().Value.Endpoint = ts.URL
	conf.TypedSpec().Value.AccessKeyId = "KEY"
	conf.TypedSpec().Value.SecretAccessKey = "SECRET"

	require.NoError(suite.T(), suite.state.Create(suite.ctx, conf))

	sf := store.NewS3StoreFactory()

	go func() {
		err := sf.Start(suite.ctx, suite.state, logger)
		if err != nil {
			panic(err)
		}
	}()

	clientMock := &talosClientMock{}

	defer clientMock.AssertExpectations(suite.T())

	clientMock.
		On(xmocks.Name((*talosClientMock).EtcdSnapshot), mock.Anything, mock.Anything, mock.Anything).
		Return(func() (io.ReadCloser, error) { return io.NopCloser(strings.NewReader("Hello World")), nil })

	suite.register2(omnictrl.NewEtcdBackupController(omnictrl.EtcdBackupControllerSettings{
		ClientMaker: func(context.Context, string) (omnictrl.TalosClient, error) {
			return clientMock, nil
		},
		StoreFactory: sf,
		Clock:        fakeclock,
		TickInterval: 10 * time.Minute,
	}))

	clusterNames := []string{"talos-default-14"}
	clusters := createClusters(suite, clusterNames, time.Hour)

	now := fakeclock.Now()

	blockAndAdvance(suite.ctx, suite.T(), fakeclock, 11*time.Minute)

	assertResource(
		&suite.OmniSuite,
		clusters[0].Metadata(),
		func(r *omni.EtcdBackupStatus, assertion *assert.Assertions) {
			value := r.TypedSpec().Value

			assertion.Zero(value.Error, "cluster %s", r.Metadata().ID())
			assertion.Equalf(specs.EtcdBackupStatusSpec_Ok, value.Status, "cluster %s", r.Metadata().ID())
			assertion.WithinRange(
				value.LastBackupTime.AsTime(),
				now,
				fakeclock.Now(),
				"cluster %s",
				r.Metadata().ID(),
			)
			assertion.WithinRange(
				value.LastBackupAttempt.AsTime(),
				now,
				fakeclock.Now(),
				"cluster %s",
				r.Metadata().ID(),
			)
		},
	)

	buckets := must.Value(backend.ListBuckets())(suite.T())

	for _, b := range buckets {
		suite.T().Logf("bucket: %s", b.Name)
	}

	it, err := must.Value(sf.GetStore())(suite.T()).ListBackups(suite.ctx, clusters[0].TypedSpec().Value.ClusterUuid)
	suite.Require().NoError(err)

	backups := toSlice(it, suite.T())
	suite.Require().Len(backups, 1)

	reader := must.Value(backups[0].Reader())(suite.T())

	suite.T().Cleanup(func() { suite.Require().NoError(reader.Close()) })

	decryptedHeader, decrypter := must.Values(crypt.Decrypt(
		reader,
		clusters[0].TypedSpec().Value.EncryptionKey,
	))(suite.T())

	suite.Require().EqualValues(
		clusters[0].TypedSpec().Value.AesCbcEncryptionSecret,
		decryptedHeader.AESCBCEncryptionSecret,
	)
	suite.Require().EqualValues(
		clusters[0].TypedSpec().Value.SecretboxEncryptionSecret,
		decryptedHeader.SecretboxEncryptionSecret,
	)
	suite.Require().EqualValues("Hello World", string(must.Value(io.ReadAll(decrypter))(suite.T())))
}

func (suite *EtcdBackupControllerSuite) fileStoreFactory() store.Factory {
	dir := filepath.Join(suite.T().TempDir(), "omni-etcd-backups")

	return store.NewFileStoreStoreFactory(dir)
}

func (suite *EtcdBackupControllerSuite) eventuallyFindBackups(sf store.Factory, clusterID string, num int) []etcdbackup.Info {
	var result []etcdbackup.Info

	suite.Require().EventuallyWithT(func(collect *assert.CollectT) {
		st, err := sf.GetStore()
		if err != nil {
			collect.Errorf("failed to get store: %s", err)

			return
		}

		bd := must.Value(safe.StateGetByID[*omni.BackupData](suite.ctx, suite.state, clusterID))(suite.T())
		it := must.Value(st.ListBackups(suite.ctx, bd.TypedSpec().Value.ClusterUuid))(suite.T())
		result = toSlice(it, suite.T())

		assert.Len(collect, result, num, "cluster %s", clusterID)
	}, 15*time.Second, 100*time.Microsecond)

	return result
}

func createClusters(suite *EtcdBackupControllerSuite, clusterNames []string, backupInterval time.Duration) []*omni.BackupData {
	return xslices.Map(clusterNames, func(clusterName string) *omni.BackupData {
		cluster, _ := suite.createCluster(clusterName, 1, 1)

		must.Value(safe.StateUpdateWithConflicts(suite.ctx, suite.state, cluster.Metadata(), func(cl *omni.Cluster) error {
			cl.TypedSpec().Value.BackupConfiguration = &specs.EtcdBackupConf{Interval: durationpb.New(backupInterval), Enabled: true}

			return nil
		}))(suite.T())

		var result *omni.BackupData

		assertResource(
			&suite.OmniSuite,
			cluster.Metadata(),
			func(r *omni.BackupData, _ *assert.Assertions) {
				result = r
			},
		)

		return result
	})
}

type talosClientMock struct{ mock.Mock }

func (t *talosClientMock) EtcdSnapshot(ctx context.Context, req *machine.EtcdSnapshotRequest, callOptions ...grpc.CallOption) (io.ReadCloser, error) {
	args := t.Called(ctx, req, callOptions)

	return xmocks.GetAs[func() (io.ReadCloser, error)](args, 0)()
}

type storeMock struct{ mock.Mock }

func (s *storeMock) ListBackups(ctx context.Context, clusterUUID string) (iter.Seq2[etcdbackup.Info, error], error) {
	args := s.Called(ctx, clusterUUID)

	return xmocks.Cast2[iter.Seq2[etcdbackup.Info, error], error](args)
}

func (s *storeMock) Upload(ctx context.Context, descr etcdbackup.Description, r io.Reader) error {
	args := s.Called(ctx, descr, r)

	return xmocks.GetAs[error](args, 0)
}

func (s *storeMock) Download(ctx context.Context, clusterUUID []byte, clusterName, snapshotName string) (etcdbackup.BackupData, io.ReadCloser, error) {
	args := s.Called(ctx, clusterUUID, clusterName, snapshotName)

	return xmocks.Cast3[etcdbackup.BackupData, io.ReadCloser, error](args)
}

type storeFactoryMock struct{ mock.Mock }

func (s *storeFactoryMock) GetStore() (etcdbackup.Store, error) {
	args := s.Called()

	return xmocks.Cast2[etcdbackup.Store, error](args)
}

func (s *storeFactoryMock) Start(ctx context.Context, st state.State, l *zap.Logger) error {
	args := s.Called(ctx, st, l)

	return xmocks.GetAs[error](args, 0)
}

func (s *storeFactoryMock) Description() string {
	return "mock store"
}

func toIter(infos []etcdbackup.Info) iter.Seq2[etcdbackup.Info, error] {
	return func(yield func(etcdbackup.Info, error) bool) {
		for _, info := range infos {
			if !yield(info, nil) {
				break
			}
		}
	}
}

func toSlice(it iter.Seq2[etcdbackup.Info, error], t *testing.T) []etcdbackup.Info {
	var result []etcdbackup.Info //nolint:prealloc

	for v, err := range it {
		require.NoError(t, err)

		result = append(result, v)
	}

	return result
}

func fakeClock() *clockwork.FakeClock {
	return clockwork.NewFakeClockAt(time.Unix(0, 0).UTC())
}

func blockAndAdvance(ctx context.Context, t *testing.T, fc *clockwork.FakeClock, duration time.Duration) {
	require.NoError(t, fc.BlockUntilContext(ctx, 1))
	fc.Advance(duration)
}

type fakeBackend struct {
	*s3mem.Backend
	logger *zap.Logger
}

func (f *fakeBackend) ListBuckets() ([]gofakes3.BucketInfo, error) {
	f.logger.Info("ListBuckets")

	return f.Backend.ListBuckets()
}

func (f *fakeBackend) BucketExists(name string) (exists bool, err error) {
	f.logger.Info("BucketExists", zap.String("name", name))

	return f.Backend.BucketExists(name)
}

func (f *fakeBackend) PutObject(bucketName, key string, meta map[string]string, input io.Reader, size int64) (gofakes3.PutObjectResult, error) {
	f.logger.Info("PutObject", zap.String("bucket_name", bucketName), zap.String("key", key), zap.Any("meta", meta), zap.Int64("size", size))

	return f.Backend.PutObject(bucketName, key, meta, input, size)
}

func newFakeBackend(logger *zap.Logger) *fakeBackend {
	return &fakeBackend{s3mem.New(), logger}
}
