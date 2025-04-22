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
	"testing/synctest"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"
	"github.com/siderolabs/gen/containers"
	"github.com/siderolabs/gen/xiter"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/gen/xtesting/must"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"
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
)

//go:generate mockgen -destination=etcd_backup_mock_1_test.go -package omni_test -typed -copyright_file ../../../../../../hack/.license-header.go.txt . TalosClient
//go:generate sed -i "s#// //#//#g" etcd_backup_mock_1_test.go

//go:generate mockgen -destination=etcd_backup_mock_2_test.go -package omni_test -typed -copyright_file ../../../../../../hack/.license-header.go.txt github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/etcdbackup/store Factory
//go:generate sed -i "s#// //#//#g" etcd_backup_mock_2_test.go

//go:generate mockgen -destination=etcd_backup_mock_3_test.go -package omni_test -typed -copyright_file ../../../../../../hack/.license-header.go.txt github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/etcdbackup Store
//go:generate sed -i "s#// //#//#g" etcd_backup_mock_3_test.go

func TestEtcdBackupControllerSuite(t *testing.T) {
	t.Parallel()

	synctest.Run(func() { suite.Run(t, new(EtcdBackupControllerSuite)) })
}

const description = "test"

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
	suite.ctx, suite.ctxCancel = context.WithCancel(context.Background())

	suite.disableConnections = true

	suite.OmniSuite.SetupTest()

	suite.startRuntime()

	suite.register(omnictrl.NewClusterController())
	suite.qregister(omnictrl.NewClusterUUIDController())
	suite.qregister(omnictrl.NewEtcdBackupEncryptionController())
	suite.qregister(omnictrl.NewSecretsController(suite.fileStoreFactory()))
	suite.qregister(omnictrl.NewBackupDataController())
}

func (suite *EtcdBackupControllerSuite) TestEtcdBackup() {
	sf := suite.fileStoreFactory()
	ctrl := gomock.NewController(suite.T())
	clientMock := NewMockTalosClient(ctrl)

	clientMock.EXPECT().
		EtcdSnapshot(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(context.Context, *machine.EtcdSnapshotRequest, ...grpc.CallOption) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("Hello World")), nil
		}).AnyTimes()

	suite.register2(omnictrl.NewEtcdBackupController(omnictrl.EtcdBackupControllerSettings{
		ClientMaker: func(context.Context, string) (omnictrl.TalosClient, error) {
			return clientMock, nil
		},
		StoreFactory: sf,
		TickInterval: 10 * time.Minute,
	}))

	clusterNames := []string{"talos-default-1", "talos-default-2"}
	clustersData := createClusters(suite, clusterNames, time.Hour)
	start := time.Now()

	// Wait for the first backup to be created.
	synctest.Wait()

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
			assertion.WithinRange(value.LastBackupTime.AsTime(), start, time.Now())
		},
	)

	// Backups should be created for both clusters since those backups do not exist yet
	for _, cd := range clustersData {
		suite.findBackups(sf, cd.Metadata().ID(), 1)
	}

	time.Sleep(time.Hour)
	synctest.Wait()

	for _, cd := range clustersData {
		suite.findBackups(sf, cd.Metadata().ID(), 2)
	}

	// Destroy the first cluster
	suite.destroyClusterByID(clustersData[0].Metadata().ID())

	time.Sleep(time.Hour)
	synctest.Wait()

	suite.findBackups(sf, clustersData[1].Metadata().ID(), 3)

	// Destroy the second cluster
	suite.destroyClusterByID(clustersData[1].Metadata().ID())
}

func (suite *EtcdBackupControllerSuite) TestEtcdBackupFactoryFails() {
	sf := suite.fileStoreFactory()
	ctrl := gomock.NewController(suite.T())
	clientMock := NewMockTalosClient(ctrl)

	clientMock.EXPECT().
		EtcdSnapshot(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(context.Context, *machine.EtcdSnapshotRequest, ...grpc.CallOption) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("Hello World")), nil
		}).
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
		TickInterval: 10 * time.Minute,
	}))

	clustersData := createClusters(suite, clusterNames, time.Hour)

	time.Sleep(11 * time.Minute)
	synctest.Wait()

	// Backups should be created for both clusters since those backups do not exist yet
	for _, cluster := range clustersData {
		suite.findBackups(sf, cluster.Metadata().ID(), 1)
	}

	start := time.Now()

	m.Set(clusterNames[0], func() (omnictrl.TalosClient, error) {
		return nil, errors.New("failed to create client")
	})

	time.Sleep(time.Hour)
	synctest.Wait()

	assertResource(
		&suite.OmniSuite,
		clustersData[0].Metadata(),
		func(r *omni.EtcdBackupStatus, assertion *assert.Assertions) {
			value := r.TypedSpec().Value

			assertion.EqualValuesf("failed to create talos client for cluster, skipping cluster backup: failed to create client", value.Error, "cluster %s", r.Metadata().ID())
			assertion.Equalf(specs.EtcdBackupStatusSpec_Error, value.Status, "cluster %s", r.Metadata().ID())
			assertion.WithinRangef(
				value.LastBackupAttempt.AsTime(),
				start,
				time.Now(),
				"cluster %s",
				r.Metadata().ID(),
			)
		},
	)

	suite.findBackups(sf, clustersData[0].Metadata().ID(), 1)
	suite.findBackups(sf, clustersData[1].Metadata().ID(), 2)

	for i := range clusterNames {
		suite.destroyClusterByID(clustersData[i].Metadata().ID())
	}
}

func (suite *EtcdBackupControllerSuite) TestDecryptEtcdBackup() {
	sf := suite.fileStoreFactory()
	ctrl := gomock.NewController(suite.T())
	clientMock := NewMockTalosClient(ctrl)

	clientMock.EXPECT().
		EtcdSnapshot(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(context.Context, *machine.EtcdSnapshotRequest, ...grpc.CallOption) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("Hello World")), nil
		})

	suite.register2(omnictrl.NewEtcdBackupController(omnictrl.EtcdBackupControllerSettings{
		ClientMaker: func(context.Context, string) (omnictrl.TalosClient, error) {
			return clientMock, nil
		},
		StoreFactory: sf,
		TickInterval: 10 * time.Minute,
	}))

	clusterNames := []string{"talos-default-6"}
	clusters := createClusters(suite, clusterNames, time.Hour)

	time.Sleep(11 * time.Minute)
	synctest.Wait()

	clusterBackups := suite.findBackups(sf, clusters[0].Metadata().ID(), 1)

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
	ctrl := gomock.NewController(suite.T())
	clientMock := NewMockTalosClient(ctrl)
	sfm := NewMockFactory(ctrl)
	sm := NewMockStore(ctrl)

	ch := make(chan string, 3)

	sfm.EXPECT().
		GetStore().
		DoAndReturn(func() (etcdbackup.Store, error) { return sm, nil }).
		AnyTimes()

	sfm.EXPECT().
		Description().
		DoAndReturn(func() string { return description }).
		AnyTimes()

	sm.EXPECT().
		ListBackups(gomock.Any(), gomock.Any()).
		Return(xiter.Empty2[etcdbackup.Info, error], nil).
		Times(2)

	sm.EXPECT().
		Upload(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(
			func(_ context.Context, description etcdbackup.Description, reader io.Reader) error {
				ch <- description.ClusterName
				must.Value(io.Copy(io.Discard, reader))(suite.T())

				return nil
			},
		).
		Times(3)

	clientMock.EXPECT().
		EtcdSnapshot(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(context.Context, *machine.EtcdSnapshotRequest, ...grpc.CallOption) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("Hello World")), nil
		}).
		AnyTimes()

	suite.register2(omnictrl.NewEtcdBackupController(omnictrl.EtcdBackupControllerSettings{
		ClientMaker: func(context.Context, string) (omnictrl.TalosClient, error) {
			return clientMock, nil
		},
		StoreFactory: sfm,
		TickInterval: 10 * time.Minute,
	}))

	clusterNames := []string{"talos-default-7", "talos-default-8"}
	clusters := createClusters(suite, clusterNames, time.Hour)

	time.Sleep(11 * time.Minute)
	synctest.Wait()

	suite.destroyClusterByID(clusters[0].Metadata().ID())

	time.Sleep(time.Hour)
	synctest.Wait()

	for i := range clusterNames {
		suite.destroyClusterByID(clusters[i].Metadata().ID())
	}
}

func (suite *EtcdBackupControllerSuite) TestListBackupsWithExistingData() {
	clusterNames := []string{"talos-default-9", "talos-default-10"}
	clusters := createClusters(suite, clusterNames, time.Hour)

	ctrl := gomock.NewController(suite.T())
	clientMock := NewMockTalosClient(ctrl)
	sfm := NewMockFactory(ctrl)
	store := NewMockStore(ctrl)

	sfm.EXPECT().
		GetStore().
		Return(store, nil).
		AnyTimes()

	sfm.EXPECT().
		Description().
		DoAndReturn(func() string { return description }).
		AnyTimes()

	store.EXPECT().
		ListBackups(gomock.Any(), gomock.Any()).
		Return(toIter([]etcdbackup.Info{
			{Timestamp: time.Now().Add(time.Minute), Reader: nil, Size: 0},
			{Timestamp: time.Now(), Reader: nil, Size: 0},
		}), nil).
		Times(2)

	store.EXPECT().
		Upload(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ etcdbackup.Description, reader io.Reader) error {
			must.Value(io.Copy(io.Discard, reader))(suite.T())

			return nil
		}).
		Times(2)

	clientMock.EXPECT().
		EtcdSnapshot(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(context.Context, *machine.EtcdSnapshotRequest, ...grpc.CallOption) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("Hello World")), nil
		}).
		AnyTimes()

	suite.register2(omnictrl.NewEtcdBackupController(omnictrl.EtcdBackupControllerSettings{
		ClientMaker: func(context.Context, string) (omnictrl.TalosClient, error) {
			return clientMock, nil
		},
		StoreFactory: sfm,
		TickInterval: 10 * time.Minute,
	}))

	time.Sleep(11 * time.Minute)
	synctest.Wait()

	suite.destroyClusterByID(clusters[0].Metadata().ID())

	time.Sleep(time.Hour)
	synctest.Wait()
	time.Sleep(time.Hour)
	synctest.Wait()

	suite.destroyClusterByID(clusters[1].Metadata().ID())
}

func (suite *EtcdBackupControllerSuite) TestEtcdManualBackupFindResource() {
	sf := suite.fileStoreFactory()
	ctrl := gomock.NewController(suite.T())
	clientMock := NewMockTalosClient(ctrl)

	suite.stateBuilder.Set(resources.ExternalNamespace, &external.State{
		CoreState:    suite.state,
		StoreFactory: sf,
	})

	clientMock.EXPECT().
		EtcdSnapshot(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(context.Context, *machine.EtcdSnapshotRequest, ...grpc.CallOption) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("Hello World")), nil
		})

	suite.register2(omnictrl.NewEtcdBackupController(omnictrl.EtcdBackupControllerSettings{
		ClientMaker: func(context.Context, string) (omnictrl.TalosClient, error) {
			return clientMock, nil
		},
		StoreFactory: sf,
		TickInterval: time.Minute,
	}))

	clusterNames := []string{"talos-default-11"}
	clustersData := createClusters(suite, clusterNames, 0) // 0 means that automatic backups are disabled

	manualBackup := omni.NewEtcdManualBackup(clustersData[0].Metadata().ID())
	manualBackup.TypedSpec().Value.BackupAt = timestamppb.New(time.Now().Add(15 * time.Second))

	err := suite.state.Create(suite.ctx, manualBackup)
	suite.Require().NoError(err)

	time.Sleep(15 * time.Second)
	synctest.Wait()

	backups := suite.findBackups(sf, clustersData[0].Metadata().ID(), 1)

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

	_, err = safe.StateList[*omni.EtcdBackup](suite.ctx, suite.state, omni.NewEtcdBackup("", time.Now()).Metadata())
	suite.Require().Error(err)

	for _, clusterData := range clustersData {
		suite.destroyClusterByID(clusterData.Metadata().ID())
	}
}

func (suite *EtcdBackupControllerSuite) TestEtcdManualBackup() {
	sf := suite.fileStoreFactory()
	ctrl := gomock.NewController(suite.T())
	clientMock := NewMockTalosClient(ctrl)

	clientMock.EXPECT().
		EtcdSnapshot(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(context.Context, *machine.EtcdSnapshotRequest, ...grpc.CallOption) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("Hello World")), nil
		})

	suite.register2(omnictrl.NewEtcdBackupController(omnictrl.EtcdBackupControllerSettings{
		ClientMaker: func(context.Context, string) (omnictrl.TalosClient, error) {
			return clientMock, nil
		},
		StoreFactory: sf,
		TickInterval: time.Minute,
	}))

	clusterNames := []string{"talos-default-12", "talos-default-13"}
	clustersData := createClusters(suite, clusterNames, 0) // 0 means that automatic backups are disabled

	time.Sleep(15 * time.Second)
	synctest.Wait()

	assertNoResource(&suite.OmniSuite, omni.NewEtcdBackupStatus(clustersData[0].Metadata().ID()))
	assertNoResource(&suite.OmniSuite, omni.NewEtcdBackupStatus(clustersData[1].Metadata().ID()))

	manualBackup := omni.NewEtcdManualBackup(clustersData[0].Metadata().ID())
	manualBackup.TypedSpec().Value.BackupAt = timestamppb.New(time.Now().Add(12 * time.Minute))

	err := suite.state.Create(suite.ctx, manualBackup)
	suite.Require().NoError(err)

	now := time.Now()

	assertNoResource(&suite.OmniSuite, omni.NewEtcdBackupStatus(clustersData[0].Metadata().ID()))
	suite.findBackups(sf, clustersData[0].Metadata().ID(), 0)

	time.Sleep(12 * time.Minute)
	synctest.Wait()

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
				time.Now(),
				"cluster %s",
				r.Metadata().ID(),
			)
			assertion.WithinRange(
				value.LastBackupAttempt.AsTime(),
				now,
				time.Now(),
				"cluster %s",
				r.Metadata().ID(),
			)
		},
	)

	suite.findBackups(sf, clustersData[0].Metadata().ID(), 1)

	time.Sleep(10 * time.Minute)
	synctest.Wait()

	// Should ignore this backup
	_, err = safe.StateUpdateWithConflicts(suite.ctx, suite.state, manualBackup.Metadata(), func(b *omni.EtcdManualBackup) error {
		b.TypedSpec().Value.BackupAt = timestamppb.New(time.Now().Add(-12 * time.Minute))

		return nil
	})
	suite.Require().NoError(err)

	suite.findBackups(sf, clustersData[0].Metadata().ID(), 1)

	for _, clusterData := range clustersData {
		suite.destroyClusterByID(clusterData.Metadata().ID())
	}
}

func (suite *EtcdBackupControllerSuite) TestS3Backup() {
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

	ctrl := gomock.NewController(suite.T())
	clientMock := NewMockTalosClient(ctrl)

	clientMock.EXPECT().
		EtcdSnapshot(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(context.Context, *machine.EtcdSnapshotRequest, ...grpc.CallOption) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("Hello World")), nil
		})

	now := time.Now()

	suite.register2(omnictrl.NewEtcdBackupController(omnictrl.EtcdBackupControllerSettings{
		ClientMaker: func(context.Context, string) (omnictrl.TalosClient, error) {
			return clientMock, nil
		},
		StoreFactory: sf,
		TickInterval: time.Minute,
	}))

	clusterNames := []string{"talos-default-14"}
	clusters := createClusters(suite, clusterNames, time.Hour)

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
				time.Now(),
				"cluster %s",
				r.Metadata().ID(),
			)
			assertion.WithinRange(
				value.LastBackupAttempt.AsTime(),
				now,
				time.Now(),
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

func (suite *EtcdBackupControllerSuite) TestBackupJitter() {
	sf := suite.fileStoreFactory()
	ctrl := gomock.NewController(suite.T())
	clientMock := NewMockTalosClient(ctrl)

	clientMock.EXPECT().
		EtcdSnapshot(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(context.Context, *machine.EtcdSnapshotRequest, ...grpc.CallOption) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("Hello World")), nil
		}).
		Times(4) // 2 clusters * 2 backups

	jitter := 20 * time.Minute

	suite.register2(omnictrl.NewEtcdBackupController(omnictrl.EtcdBackupControllerSettings{
		ClientMaker: func(context.Context, string) (omnictrl.TalosClient, error) {
			return clientMock, nil
		},
		StoreFactory: sf,
		TickInterval: 1 * time.Minute, // smaller tick, more chances for jitter to be applied
		Jitter:       jitter,
	}))

	clusterNames := []string{"talos-default-1", "talos-default-2"}
	clustersData := createClusters(suite, clusterNames, time.Hour)

	synctest.Wait()                        // Wait for the first backup to be taken, it will be without jitter
	time.Sleep(time.Hour + 10*time.Minute) // Sleep until the next backup is due

	now := time.Now().UTC()

	synctest.Wait() // Wait for the second backup to be taken, this one will have jitter

	for _, cd := range clustersData {
		st := must.Value(sf.GetStore())(suite.T())
		backups := must.Value(st.ListBackups(suite.ctx, cd.TypedSpec().Value.ClusterUuid))(suite.T())
		slc := toSlice(backups, suite.T())

		suite.Require().Len(slc, 2)

		for _, b := range slc[:1] { // we are only interested in the last backup (reverse order)
			suite.Require().NotEqual(now, b.Timestamp.UTC())
			suite.Require().WithinDuration(now, b.Timestamp, jitter)
		}
	}

	// Destroy the clusters
	for _, clusterData := range clustersData {
		suite.destroyClusterByID(clusterData.Metadata().ID())
	}
}

func (suite *EtcdBackupControllerSuite) fileStoreFactory() store.Factory {
	dir := filepath.Join(suite.T().TempDir(), "omni-etcd-backups")

	return store.NewFileStoreStoreFactory(dir)
}

func (suite *EtcdBackupControllerSuite) findBackups(sf store.Factory, clusterID string, num int) []etcdbackup.Info {
	st := must.Value(sf.GetStore())(suite.T())
	bd := must.Value(safe.StateGetByID[*omni.BackupData](suite.ctx, suite.state, clusterID))(suite.T())
	it := must.Value(st.ListBackups(suite.ctx, bd.TypedSpec().Value.ClusterUuid))(suite.T())
	result := toSlice(it, suite.T())

	suite.Require().Len(result, num, "cluster %s", clusterID)

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
