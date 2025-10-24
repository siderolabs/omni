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

	"github.com/cosi-project/runtime/pkg/controller/runtime"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"
	"github.com/siderolabs/gen/containers"
	"github.com/siderolabs/gen/xiter"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/gen/xtesting/must"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	omniruntime "github.com/siderolabs/omni/internal/backend/runtime"
	"github.com/siderolabs/omni/internal/backend/runtime/kubernetes"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/etcdbackup"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/etcdbackup/store"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/etcdbackup/crypt"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/external"
)

//go:generate mockgen -destination=etcd_backup_mock_1_test.go -package omni_test -typed -copyright_file ../../../../../../hack/.license-header.go.txt . TalosClient
//go:generate sed -i "s#// //#//#g" etcd_backup_mock_1_test.go

//go:generate mockgen -destination=etcd_backup_mock_2_test.go -package omni_test -typed -copyright_file ../../../../../../hack/.license-header.go.txt github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/etcdbackup/store Factory
//go:generate sed -i "s#// //#//#g" etcd_backup_mock_2_test.go

//go:generate mockgen -destination=etcd_backup_mock_3_test.go -package omni_test -typed -copyright_file ../../../../../../hack/.license-header.go.txt github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/etcdbackup Store
//go:generate sed -i "s#// //#//#g" etcd_backup_mock_3_test.go

const description = "test"

func beforeStart(st state.State, t *testing.T, rt *runtime.Runtime, fileStoreStoreFactory store.Factory) {
	k8s, err := kubernetes.NewWithTTL(st, 0)
	require.NoError(t, err)

	omniruntime.Install(kubernetes.Name, k8s)

	require.NoError(t, rt.RegisterController(omnictrl.NewClusterController()))
	require.NoError(t, rt.RegisterQController(omnictrl.NewClusterUUIDController()))
	require.NoError(t, rt.RegisterQController(omnictrl.NewEtcdBackupEncryptionController()))
	require.NoError(t, rt.RegisterQController(omnictrl.NewSecretsController(fileStoreStoreFactory)))
	require.NoError(t, rt.RegisterQController(omnictrl.NewBackupDataController()))
}

func TestEtcdBackup(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "omni-etcd-backups")
	fileStoreStoreFactory := store.NewFileStoreStoreFactory(dir)
	sb := testutils.DynamicStateBuilder{M: map[resource.Namespace]state.CoreState{}}

	synctest.Test(t, func(t *testing.T) {
		testutils.WithRuntime(
			t.Context(),
			t,
			sb.Builder,
			func(_ context.Context, st state.State, rt *runtime.Runtime, _ *zap.Logger) { // prepare - register controllers
				beforeStart(st, t, rt, fileStoreStoreFactory)
			},
			func(ctx context.Context, st state.State, rt *runtime.Runtime, _ *zap.Logger) {
				testEtcdBackup(ctx, t, rt, st, fileStoreStoreFactory)
			},
		)
	})
}

func testEtcdBackup(ctx context.Context, t *testing.T, rt *runtime.Runtime, st state.State, sf store.Factory) {
	ctrl := gomock.NewController(t)
	clientMock := NewMockTalosClient(ctrl)

	clientMock.EXPECT().
		EtcdSnapshot(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(context.Context, *machine.EtcdSnapshotRequest, ...grpc.CallOption) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("Hello World")), nil
		}).AnyTimes()

	etcdBackupController, err := omnictrl.NewEtcdBackupController(omnictrl.EtcdBackupControllerSettings{
		ClientMaker: func(context.Context, string) (omnictrl.TalosClient, error) {
			return clientMock, nil
		},
		StoreFactory: sf,
		TickInterval: 10 * time.Minute,
	})
	require.NoError(t, err)

	require.NoError(t, rt.RegisterController(etcdBackupController))

	clusterNames := []string{"talos-default-1", "talos-default-2"}

	clustersData := createClusters(ctx, t, clusterNames, st, time.Hour)

	start := time.Now()

	// Wait for the first backup to be created.
	synctest.Wait()

	rtestutils.AssertResources(
		ctx,
		t,
		st,
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
		findBackups(ctx, t, st, sf, cd.Metadata().ID(), 1)
	}

	time.Sleep(time.Hour)
	synctest.Wait()

	for _, cd := range clustersData {
		findBackups(ctx, t, st, sf, cd.Metadata().ID(), 2)
	}

	// Destroy the first cluster
	rtestutils.Destroy[*omni.Cluster](ctx, t, st, []string{clustersData[0].Metadata().ID()})

	time.Sleep(time.Hour)
	synctest.Wait()

	findBackups(ctx, t, st, sf, clustersData[1].Metadata().ID(), 3)

	rtestutils.Destroy[*omni.Cluster](ctx, t, st, []string{clustersData[1].Metadata().ID()})
}

func TestEtcdBackupFactoryFails(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "omni-etcd-backups")
	fileStoreStoreFactory := store.NewFileStoreStoreFactory(dir)
	sb := testutils.DynamicStateBuilder{M: map[resource.Namespace]state.CoreState{}}

	synctest.Test(t, func(t *testing.T) {
		testutils.WithRuntime(
			t.Context(),
			t,
			sb.Builder,
			func(_ context.Context, st state.State, rt *runtime.Runtime, _ *zap.Logger) { // prepare - register controllers
				beforeStart(st, t, rt, fileStoreStoreFactory)
			},
			func(ctx context.Context, st state.State, rt *runtime.Runtime, _ *zap.Logger) {
				testEtcdBackupFactoryFails(ctx, t, rt, st, fileStoreStoreFactory)
			},
		)
	})
}

func testEtcdBackupFactoryFails(ctx context.Context, t *testing.T, rt *runtime.Runtime, st state.State, sf store.Factory) {
	ctrl := gomock.NewController(t)
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

	etcdBackupController, err := omnictrl.NewEtcdBackupController(omnictrl.EtcdBackupControllerSettings{
		ClientMaker: func(_ context.Context, clusterName string) (omnictrl.TalosClient, error) {
			fn, _ := m.Get(clusterName)

			return fn()
		},
		StoreFactory: sf,
		TickInterval: 10 * time.Minute,
	})
	require.NoError(t, err)
	require.NoError(t, rt.RegisterController(etcdBackupController))

	clustersData := createClusters(ctx, t, clusterNames, st, time.Hour)

	time.Sleep(11 * time.Minute)
	synctest.Wait()

	// Backups should be created for both clusters since those backups do not exist yet
	for _, cluster := range clustersData {
		findBackups(ctx, t, st, sf, cluster.Metadata().ID(), 1)
	}

	start := time.Now()

	m.Set(clusterNames[0], func() (omnictrl.TalosClient, error) {
		return nil, errors.New("failed to create client")
	})

	time.Sleep(time.Hour)
	synctest.Wait()

	rtestutils.AssertResources(
		ctx,
		t,
		st,
		[]resource.ID{clustersData[0].Metadata().ID()},
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

	findBackups(ctx, t, st, sf, clustersData[0].Metadata().ID(), 1)
	findBackups(ctx, t, st, sf, clustersData[1].Metadata().ID(), 2)

	for i := range clusterNames {
		rtestutils.Destroy[*omni.Cluster](ctx, t, st, []string{clustersData[i].Metadata().ID()})
	}
}

func TestDecryptEtcdBackup(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "omni-etcd-backups")
	fileStoreStoreFactory := store.NewFileStoreStoreFactory(dir)
	sb := testutils.DynamicStateBuilder{M: map[resource.Namespace]state.CoreState{}}

	synctest.Test(t, func(t *testing.T) {
		testutils.WithRuntime(
			t.Context(),
			t,
			sb.Builder,
			func(_ context.Context, st state.State, rt *runtime.Runtime, _ *zap.Logger) { // prepare - register controllers
				beforeStart(st, t, rt, fileStoreStoreFactory)
			},
			func(ctx context.Context, st state.State, rt *runtime.Runtime, _ *zap.Logger) {
				testDecryptEtcdBackup(ctx, t, rt, st, fileStoreStoreFactory)
			},
		)
	})
}

func testDecryptEtcdBackup(ctx context.Context, t *testing.T, rt *runtime.Runtime, st state.State, sf store.Factory) {
	ctrl := gomock.NewController(t)
	clientMock := NewMockTalosClient(ctrl)

	clientMock.EXPECT().
		EtcdSnapshot(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(context.Context, *machine.EtcdSnapshotRequest, ...grpc.CallOption) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("Hello World")), nil
		})

	etcdBackupController, err := omnictrl.NewEtcdBackupController(omnictrl.EtcdBackupControllerSettings{
		ClientMaker: func(context.Context, string) (omnictrl.TalosClient, error) {
			return clientMock, nil
		},
		StoreFactory: sf,
		TickInterval: 10 * time.Minute,
	})
	require.NoError(t, err)
	require.NoError(t, rt.RegisterController(etcdBackupController))

	clusterNames := []string{"talos-default-6"}
	clusters := createClusters(ctx, t, clusterNames, st, time.Hour)

	time.Sleep(11 * time.Minute)
	synctest.Wait()

	clusterBackups := findBackups(ctx, t, st, sf, clusters[0].Metadata().ID(), 1)

	src := must.Value(clusterBackups[0].Reader())(t)

	t.Cleanup(func() { require.NoError(t, src.Close()) })

	decryptedHeader, decrypter := must.Values(crypt.Decrypt(
		src,
		clusters[0].TypedSpec().Value.EncryptionKey,
	))(t)

	require.EqualValues(t,
		clusters[0].TypedSpec().Value.AesCbcEncryptionSecret,
		decryptedHeader.AESCBCEncryptionSecret,
	)
	require.EqualValues(t,
		clusters[0].TypedSpec().Value.SecretboxEncryptionSecret,
		decryptedHeader.SecretboxEncryptionSecret,
	)
	require.EqualValues(t, "Hello World", string(must.Value(io.ReadAll(decrypter))(t)))

	for i := range clusterNames {
		rtestutils.Destroy[*omni.Cluster](ctx, t, st, []string{clusters[i].Metadata().ID()})
	}
}

func TestSingleListCall(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "omni-etcd-backups")
	fileStoreStoreFactory := store.NewFileStoreStoreFactory(dir)
	sb := testutils.DynamicStateBuilder{M: map[resource.Namespace]state.CoreState{}}

	synctest.Test(t, func(t *testing.T) {
		testutils.WithRuntime(
			t.Context(),
			t,
			sb.Builder,
			func(_ context.Context, st state.State, rt *runtime.Runtime, _ *zap.Logger) { // prepare - register controllers
				beforeStart(st, t, rt, fileStoreStoreFactory)
			},
			func(ctx context.Context, st state.State, rt *runtime.Runtime, _ *zap.Logger) {
				testSingleListCall(ctx, t, rt, st)
			},
		)
	})
}

func testSingleListCall(ctx context.Context, t *testing.T, rt *runtime.Runtime, st state.State) {
	ctrl := gomock.NewController(t)
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

				must.Value(io.Copy(io.Discard, reader))(t)

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

	etcdBackupController, err := omnictrl.NewEtcdBackupController(omnictrl.EtcdBackupControllerSettings{
		ClientMaker: func(context.Context, string) (omnictrl.TalosClient, error) {
			return clientMock, nil
		},
		StoreFactory: sfm,
		TickInterval: 10 * time.Minute,
	})
	require.NoError(t, err)
	require.NoError(t, rt.RegisterController(etcdBackupController))

	clusterNames := []string{"talos-default-7", "talos-default-8"}
	clusters := createClusters(ctx, t, clusterNames, st, time.Hour)

	time.Sleep(11 * time.Minute)
	synctest.Wait()

	rtestutils.Destroy[*omni.Cluster](ctx, t, st, []string{clusters[0].Metadata().ID()})

	time.Sleep(time.Hour)
	synctest.Wait()

	for i := range clusterNames {
		rtestutils.Destroy[*omni.Cluster](ctx, t, st, []string{clusters[i].Metadata().ID()})
	}
}

func TestListBackupsWithExistingData(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "omni-etcd-backups")
	fileStoreStoreFactory := store.NewFileStoreStoreFactory(dir)
	sb := testutils.DynamicStateBuilder{M: map[resource.Namespace]state.CoreState{}}

	synctest.Test(t, func(t *testing.T) {
		testutils.WithRuntime(
			t.Context(),
			t,
			sb.Builder,
			func(_ context.Context, st state.State, rt *runtime.Runtime, _ *zap.Logger) { // prepare - register controllers
				beforeStart(st, t, rt, fileStoreStoreFactory)
			},
			func(ctx context.Context, st state.State, rt *runtime.Runtime, _ *zap.Logger) {
				testListBackupsWithExistingData(ctx, t, rt, st)
			},
		)
	})
}

func testListBackupsWithExistingData(ctx context.Context, t *testing.T, rt *runtime.Runtime, st state.State) {
	clusterNames := []string{"talos-default-9", "talos-default-10"}
	clusters := createClusters(ctx, t, clusterNames, st, time.Hour)

	ctrl := gomock.NewController(t)
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
			must.Value(io.Copy(io.Discard, reader))(t)

			return nil
		}).
		Times(2)

	clientMock.EXPECT().
		EtcdSnapshot(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(context.Context, *machine.EtcdSnapshotRequest, ...grpc.CallOption) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("Hello World")), nil
		}).
		AnyTimes()

	etcdBackupController, err := omnictrl.NewEtcdBackupController(omnictrl.EtcdBackupControllerSettings{
		ClientMaker: func(context.Context, string) (omnictrl.TalosClient, error) {
			return clientMock, nil
		},
		StoreFactory: sfm,
		TickInterval: 10 * time.Minute,
	})
	require.NoError(t, err)
	require.NoError(t, rt.RegisterController(etcdBackupController))

	time.Sleep(11 * time.Minute)
	synctest.Wait()

	rtestutils.Destroy[*omni.Cluster](ctx, t, st, []string{clusters[0].Metadata().ID()})

	time.Sleep(time.Hour)
	synctest.Wait()
	time.Sleep(time.Hour)
	synctest.Wait()

	rtestutils.Destroy[*omni.Cluster](ctx, t, st, []string{clusters[1].Metadata().ID()})
}

func TestEtcdManualBackupFindResource(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "omni-etcd-backups")
	fileStoreStoreFactory := store.NewFileStoreStoreFactory(dir)
	stateBuilder := testutils.DynamicStateBuilder{M: map[resource.Namespace]state.CoreState{}}
	stateBuilder.Set(resources.ExternalNamespace, &external.State{
		CoreState:    state.WrapCore(namespaced.NewState(stateBuilder.Builder)),
		StoreFactory: fileStoreStoreFactory,
	})

	synctest.Test(t, func(t *testing.T) {
		testutils.WithRuntime(
			t.Context(),
			t,
			stateBuilder.Builder,
			func(_ context.Context, st state.State, rt *runtime.Runtime, _ *zap.Logger) { // prepare - register controllers
				beforeStart(st, t, rt, fileStoreStoreFactory)
			},
			func(ctx context.Context, st state.State, rt *runtime.Runtime, _ *zap.Logger) {
				testEtcdManualBackupFindResource(ctx, t, rt, st, fileStoreStoreFactory)
			},
		)
	})
}

func testEtcdManualBackupFindResource(ctx context.Context, t *testing.T, rt *runtime.Runtime, st state.State, sf store.Factory) {
	ctrl := gomock.NewController(t)
	clientMock := NewMockTalosClient(ctrl)

	clientMock.EXPECT().
		EtcdSnapshot(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(context.Context, *machine.EtcdSnapshotRequest, ...grpc.CallOption) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("Hello World")), nil
		})

	etcdBackupController, err := omnictrl.NewEtcdBackupController(omnictrl.EtcdBackupControllerSettings{
		ClientMaker: func(context.Context, string) (omnictrl.TalosClient, error) {
			return clientMock, nil
		},
		StoreFactory: sf,
		TickInterval: time.Minute,
	})
	require.NoError(t, err)
	require.NoError(t, rt.RegisterController(etcdBackupController))

	clusterNames := []string{"talos-default-11"}
	clustersData := createClusters(ctx, t, clusterNames, st, 0) // 0 means that automatic backups are disabled

	manualBackup := omni.NewEtcdManualBackup(clustersData[0].Metadata().ID())
	manualBackup.TypedSpec().Value.BackupAt = timestamppb.New(time.Now().Add(15 * time.Second))

	err = st.Create(ctx, manualBackup)
	require.NoError(t, err)

	time.Sleep(15 * time.Second)
	synctest.Wait()

	backups := findBackups(ctx, t, st, sf, clustersData[0].Metadata().ID(), 1)

	// Should find backups by cluster ID

	backupRes := must.Value(safe.StateListAll[*omni.EtcdBackup](
		ctx,
		st,
		state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, clusterNames[0])),
	))(t)

	assert.EqualValues(t, len(backups), backupRes.Len())

	assert.Equal(t,
		omni.NewEtcdBackup(clusterNames[0], backups[0].Timestamp).Metadata().ID(),
		backupRes.Get(0).Metadata().ID(),
	)

	assert.Equal(t, backups[0].Snapshot, backupRes.Get(0).TypedSpec().Value.Snapshot)
	assert.Equal(t, backups[0].Timestamp.UTC(), backupRes.Get(0).TypedSpec().Value.CreatedAt.AsTime())

	// Should find backup by backup id

	res := must.Value(safe.StateGetByID[*omni.EtcdBackup](ctx, st, backupRes.Get(0).Metadata().ID()))(t)

	assert.Equal(t, backups[0].Snapshot, res.TypedSpec().Value.Snapshot)
	assert.Equal(t, backups[0].Timestamp.UTC(), res.TypedSpec().Value.CreatedAt.AsTime())

	// Should return an error if no query is provided

	_, err = safe.StateList[*omni.EtcdBackup](ctx, st, omni.NewEtcdBackup("", time.Now()).Metadata())
	require.Error(t, err)

	for _, clusterData := range clustersData {
		rtestutils.Destroy[*omni.Cluster](ctx, t, st, []string{clusterData.Metadata().ID()})
	}
}

func TestEtcdManualBackup(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "omni-etcd-backups")
	fileStoreStoreFactory := store.NewFileStoreStoreFactory(dir)
	sb := testutils.DynamicStateBuilder{M: map[resource.Namespace]state.CoreState{}}

	synctest.Test(t, func(t *testing.T) {
		testutils.WithRuntime(
			t.Context(),
			t,
			sb.Builder,
			func(_ context.Context, st state.State, rt *runtime.Runtime, _ *zap.Logger) { // prepare - register controllers
				beforeStart(st, t, rt, fileStoreStoreFactory)
			},
			func(ctx context.Context, st state.State, rt *runtime.Runtime, _ *zap.Logger) {
				testEtcdManualBackup(ctx, t, rt, st, fileStoreStoreFactory)
			},
		)
	})
}

func testEtcdManualBackup(ctx context.Context, t *testing.T, rt *runtime.Runtime, st state.State, sf store.Factory) {
	ctrl := gomock.NewController(t)
	clientMock := NewMockTalosClient(ctrl)

	clientMock.EXPECT().
		EtcdSnapshot(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(context.Context, *machine.EtcdSnapshotRequest, ...grpc.CallOption) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("Hello World")), nil
		})

	etcdBackupController, err := omnictrl.NewEtcdBackupController(omnictrl.EtcdBackupControllerSettings{
		ClientMaker: func(context.Context, string) (omnictrl.TalosClient, error) {
			return clientMock, nil
		},
		StoreFactory: sf,
		TickInterval: time.Minute,
	})
	require.NoError(t, err)
	require.NoError(t, rt.RegisterController(etcdBackupController))

	clusterNames := []string{"talos-default-12", "talos-default-13"}
	clustersData := createClusters(ctx, t, clusterNames, st, 0) // 0 means that automatic backups are disabled

	time.Sleep(15 * time.Second)
	synctest.Wait()

	rtestutils.AssertNoResource[*omni.EtcdBackupStatus](ctx, t, st, omni.NewEtcdBackupStatus(clustersData[0].Metadata().ID()).Metadata().ID())
	rtestutils.AssertNoResource[*omni.EtcdBackupStatus](ctx, t, st, omni.NewEtcdBackupStatus(clustersData[1].Metadata().ID()).Metadata().ID())

	manualBackup := omni.NewEtcdManualBackup(clustersData[0].Metadata().ID())
	manualBackup.TypedSpec().Value.BackupAt = timestamppb.New(time.Now().Add(12 * time.Minute))

	err = st.Create(ctx, manualBackup)
	require.NoError(t, err)

	now := time.Now()

	rtestutils.AssertNoResource[*omni.EtcdBackupStatus](ctx, t, st, omni.NewEtcdBackupStatus(clustersData[0].Metadata().ID()).Metadata().ID())
	findBackups(ctx, t, st, sf, clustersData[0].Metadata().ID(), 0)

	time.Sleep(12 * time.Minute)
	synctest.Wait()
	rtestutils.AssertResource(
		ctx,
		t,
		st,
		clustersData[0].Metadata().ID(),
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

	findBackups(ctx, t, st, sf, clustersData[0].Metadata().ID(), 1)

	time.Sleep(10 * time.Minute)
	synctest.Wait()

	// Should ignore this backup
	_, err = safe.StateUpdateWithConflicts(ctx, st, manualBackup.Metadata(), func(b *omni.EtcdManualBackup) error {
		b.TypedSpec().Value.BackupAt = timestamppb.New(time.Now().Add(-12 * time.Minute))

		return nil
	})
	require.NoError(t, err)

	findBackups(ctx, t, st, sf, clustersData[0].Metadata().ID(), 1)

	for _, clusterData := range clustersData {
		rtestutils.Destroy[*omni.Cluster](ctx, t, st, []string{clusterData.Metadata().ID()})
	}
}

func TestS3Backup(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "omni-etcd-backups")
	fileStoreStoreFactory := store.NewFileStoreStoreFactory(dir)
	sb := testutils.DynamicStateBuilder{M: map[resource.Namespace]state.CoreState{}}

	synctest.Test(t, func(t *testing.T) {
		testutils.WithRuntime(
			t.Context(),
			t,
			sb.Builder,
			func(_ context.Context, st state.State, rt *runtime.Runtime, _ *zap.Logger) { // prepare - register controllers
				beforeStart(st, t, rt, fileStoreStoreFactory)
			},
			func(ctx context.Context, st state.State, rt *runtime.Runtime, _ *zap.Logger) {
				testS3Backup(ctx, t, rt, st)
			},
		)
	})
}

func testS3Backup(ctx context.Context, t *testing.T, rt *runtime.Runtime, st state.State) {
	logger := zaptest.NewLogger(t)

	backend := newFakeBackend(logger)
	faker := gofakes3.New(backend)

	ts := httptest.NewServer(faker.Server())
	defer ts.Close()

	const bucket = "test-bucket"

	require.NoError(t, backend.CreateBucket(bucket))

	conf := omni.NewEtcdBackupS3Conf()
	conf.TypedSpec().Value.Bucket = bucket
	conf.TypedSpec().Value.Region = "us-east-1"
	conf.TypedSpec().Value.Endpoint = ts.URL
	conf.TypedSpec().Value.AccessKeyId = "KEY"
	conf.TypedSpec().Value.SecretAccessKey = "SECRET"

	require.NoError(t, st.Create(ctx, conf))

	sf := store.NewS3StoreFactory()

	go func() {
		err := sf.Start(ctx, st, logger)
		if err != nil {
			panic(err)
		}
	}()

	ctrl := gomock.NewController(t)
	clientMock := NewMockTalosClient(ctrl)

	clientMock.EXPECT().
		EtcdSnapshot(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(context.Context, *machine.EtcdSnapshotRequest, ...grpc.CallOption) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("Hello World")), nil
		})

	now := time.Now()

	etcdBackupController, err := omnictrl.NewEtcdBackupController(omnictrl.EtcdBackupControllerSettings{
		ClientMaker: func(context.Context, string) (omnictrl.TalosClient, error) {
			return clientMock, nil
		},
		StoreFactory: sf,
		TickInterval: time.Minute,
	})
	require.NoError(t, err)
	require.NoError(t, rt.RegisterController(etcdBackupController))

	clusterNames := []string{"talos-default-14"}
	clusters := createClusters(ctx, t, clusterNames, st, time.Hour)

	rtestutils.AssertResource(
		ctx,
		t,
		st,
		clusters[0].Metadata().ID(),
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

	buckets := must.Value(backend.ListBuckets())(t)

	for _, b := range buckets {
		t.Logf("bucket: %s", b.Name)
	}

	it, err := must.Value(sf.GetStore())(t).ListBackups(ctx, clusters[0].TypedSpec().Value.ClusterUuid)
	require.NoError(t, err)

	backups := toSlice(t, it)
	require.Len(t, backups, 1)

	reader := must.Value(backups[0].Reader())(t)

	t.Cleanup(func() { require.NoError(t, reader.Close()) })

	decryptedHeader, decrypter := must.Values(crypt.Decrypt(
		reader,
		clusters[0].TypedSpec().Value.EncryptionKey,
	))(t)

	require.EqualValues(t,
		clusters[0].TypedSpec().Value.AesCbcEncryptionSecret,
		decryptedHeader.AESCBCEncryptionSecret,
	)
	require.EqualValues(t,
		clusters[0].TypedSpec().Value.SecretboxEncryptionSecret,
		decryptedHeader.SecretboxEncryptionSecret,
	)
	require.EqualValues(t, "Hello World", string(must.Value(io.ReadAll(decrypter))(t)))
}

func TestBackupJitter(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "omni-etcd-backups")
	fileStoreStoreFactory := store.NewFileStoreStoreFactory(dir)
	sb := testutils.DynamicStateBuilder{M: map[resource.Namespace]state.CoreState{}}

	synctest.Test(t, func(t *testing.T) {
		testutils.WithRuntime(
			t.Context(),
			t,
			sb.Builder,
			func(_ context.Context, st state.State, rt *runtime.Runtime, _ *zap.Logger) { // prepare - register controllers
				beforeStart(st, t, rt, fileStoreStoreFactory)
			},
			func(ctx context.Context, st state.State, rt *runtime.Runtime, _ *zap.Logger) {
				testBackupJitter(ctx, t, rt, st, fileStoreStoreFactory)
			},
		)
	})
}

func testBackupJitter(ctx context.Context, t *testing.T, rt *runtime.Runtime, st state.State, sf store.Factory) {
	ctrl := gomock.NewController(t)
	clientMock := NewMockTalosClient(ctrl)

	clientMock.EXPECT().
		EtcdSnapshot(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(context.Context, *machine.EtcdSnapshotRequest, ...grpc.CallOption) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("Hello World")), nil
		}).
		Times(4) // 2 clusters * 2 backups

	jitter := 20 * time.Minute
	etcdBackupController, err := omnictrl.NewEtcdBackupController(omnictrl.EtcdBackupControllerSettings{
		ClientMaker: func(context.Context, string) (omnictrl.TalosClient, error) {
			return clientMock, nil
		},
		StoreFactory: sf,
		TickInterval: 1 * time.Minute, // smaller tick, more chances for jitter to be applied
		Jitter:       jitter,
	})
	require.NoError(t, err)
	require.NoError(t, rt.RegisterController(etcdBackupController))

	clusterNames := []string{"talos-default-1", "talos-default-2"}
	clustersData := createClusters(ctx, t, clusterNames, st, time.Hour)

	synctest.Wait()                        // Wait for the first backup to be taken, it will be without jitter
	time.Sleep(time.Hour + 10*time.Minute) // Sleep until the next backup is due

	now := time.Now().UTC()

	synctest.Wait() // Wait for the second backup to be taken, this one will have jitter

	for _, cd := range clustersData {
		st := must.Value(sf.GetStore())(t)
		backups := must.Value(st.ListBackups(ctx, cd.TypedSpec().Value.ClusterUuid))(t)
		slc := toSlice(t, backups)

		require.Len(t, slc, 2)

		for _, b := range slc[:1] { // we are only interested in the last backup (reverse order)
			require.NotEqual(t, now, b.Timestamp.UTC())
			require.WithinDuration(t, now, b.Timestamp, jitter)
		}
	}

	// Destroy the clusters
	for _, clusterData := range clustersData {
		rtestutils.Destroy[*omni.Cluster](ctx, t, st, []string{clusterData.Metadata().ID()})
	}
}

func createClusters(ctx context.Context, t *testing.T, clusterNames []string, st state.State, backupInterval time.Duration) []*omni.BackupData {
	clustersData := make([]*omni.BackupData, 0, len(clusterNames))

	for _, name := range clusterNames {
		cluster := omni.NewCluster(resources.DefaultNamespace, name)
		cluster.TypedSpec().Value.TalosVersion = "1.3.0"
		cluster.TypedSpec().Value.BackupConfiguration = &specs.EtcdBackupConf{Interval: durationpb.New(backupInterval), Enabled: true}

		require.NoError(t, st.Create(ctx, cluster))
		require.NoError(t, st.Create(ctx, omni.NewMachineSet(resources.DefaultNamespace, omni.ControlPlanesResourceID(name))))

		var backupData *omni.BackupData

		rtestutils.AssertResource[*omni.BackupData](ctx, t, st, name, func(r *omni.BackupData, assertion *assert.Assertions) {
			backupData = r
		})

		clustersData = append(clustersData, backupData)
	}

	return clustersData
}

func findBackups(ctx context.Context, t *testing.T, st state.State, sf store.Factory, clusterID string, num int) []etcdbackup.Info {
	backupStore := must.Value(sf.GetStore())(t)
	bd := must.Value(safe.StateGetByID[*omni.BackupData](ctx, st, clusterID))(t)
	it := must.Value(backupStore.ListBackups(ctx, bd.TypedSpec().Value.ClusterUuid))(t)
	result := toSlice(t, it)

	require.Len(t, result, num, "cluster %s", clusterID)

	return result
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

func toSlice(t *testing.T, it iter.Seq2[etcdbackup.Info, error]) []etcdbackup.Info {
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

func (f *fakeBackend) PutObject(bucketName, key string, meta map[string]string, input io.Reader, size int64, conditions *gofakes3.PutConditions) (gofakes3.PutObjectResult, error) {
	f.logger.Info("PutObject", zap.String("bucket_name", bucketName), zap.String("key", key), zap.Any("meta", meta), zap.Int64("size", size))

	return f.Backend.PutObject(bucketName, key, meta, input, size, conditions)
}

func newFakeBackend(logger *zap.Logger) *fakeBackend {
	return &fakeBackend{s3mem.New(), logger}
}
