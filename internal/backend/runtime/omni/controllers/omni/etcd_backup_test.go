// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"context"
	_ "embed"
	"errors"
	"io"
	"iter"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
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
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/gen/xtesting/must"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/kubernetes"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/etcdbackup"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/etcdbackup/store"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/etcdbackup/crypt"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/external"
)

// secretsJSON is the secrets used in tests to avoid the expensive secret bundle creation in SecretsController in the tests.
//
//go:embed testdata/secrets.json
var secretsJSON []byte

func beforeStart(st state.State, t *testing.T, rt *runtime.Runtime) {
	kubernetesRuntime, err := kubernetes.NewWithTTL(st, 0)
	require.NoError(t, err)

	require.NoError(t, rt.RegisterController(omnictrl.NewClusterController(kubernetesRuntime)))
	require.NoError(t, rt.RegisterQController(omnictrl.NewClusterUUIDController()))
	require.NoError(t, rt.RegisterQController(omnictrl.NewEtcdBackupEncryptionController()))
	require.NoError(t, rt.RegisterQController(omnictrl.NewBackupDataController()))
}

func TestEtcdBackup(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		testutils.WithRuntime(
			t.Context(),
			t,
			testutils.TestOptions{
				DisableCache: true,
			},
			func(_ context.Context, testContext testutils.TestContext) { // prepare - register controllers
				beforeStart(testContext.State, t, testContext.Runtime)
			},
			func(ctx context.Context, testContext testutils.TestContext) {
				testEtcdBackup(ctx, t, testContext)
			},
		)
	})
}

func testEtcdBackup(ctx context.Context, t *testing.T, testContext testutils.TestContext) {
	factory := startFactory(ctx, t, store.NewFileStoreStoreFactory(t.TempDir()), testContext.State, testContext.Logger)
	clientMock := &mockTalosClient{}
	st := testContext.State

	etcdBackupController, err := omnictrl.NewEtcdBackupController(omnictrl.EtcdBackupControllerSettings{
		ClientMaker: func(context.Context, string) (omnictrl.TalosClient, error) {
			return clientMock, nil
		},
		StoreFactory: factory,
		TickInterval: 10 * time.Minute,
	})
	require.NoError(t, err)

	require.NoError(t, testContext.Runtime.RegisterController(etcdBackupController))

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
		findBackups(ctx, t, st, factory, cd.Metadata().ID(), 1)
	}

	time.Sleep(time.Hour)
	synctest.Wait()

	for _, cd := range clustersData {
		findBackups(ctx, t, st, factory, cd.Metadata().ID(), 2)
	}

	// Destroy the first cluster
	rtestutils.Destroy[*omni.Cluster](ctx, t, st, []string{clustersData[0].Metadata().ID()})

	time.Sleep(time.Hour)
	synctest.Wait()

	findBackups(ctx, t, st, factory, clustersData[1].Metadata().ID(), 3)

	rtestutils.Destroy[*omni.Cluster](ctx, t, st, []string{clustersData[1].Metadata().ID()})
}

func TestEtcdBackupFactoryFails(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		testutils.WithRuntime(
			t.Context(),
			t,
			testutils.TestOptions{
				DisableCache: true,
			},
			func(_ context.Context, testContext testutils.TestContext) { // prepare - register controllers
				beforeStart(testContext.State, t, testContext.Runtime)
			},
			func(ctx context.Context, testContext testutils.TestContext) {
				testEtcdBackupFactoryFails(ctx, t, testContext)
			},
		)
	})
}

func testEtcdBackupFactoryFails(ctx context.Context, t *testing.T, testContext testutils.TestContext) {
	factory := startFactory(ctx, t, store.NewFileStoreStoreFactory(t.TempDir()), testContext.State, testContext.Logger)
	talosClient := &mockTalosClient{}

	var m containers.ConcurrentMap[string, func() (omnictrl.TalosClient, error)]

	clusterNames := []string{"talos-default-3", "talos-default-4", "talos-default-5"}

	for _, clusterName := range clusterNames {
		m.Set(clusterName, func() (omnictrl.TalosClient, error) {
			return talosClient, nil
		})
	}

	etcdBackupController, err := omnictrl.NewEtcdBackupController(omnictrl.EtcdBackupControllerSettings{
		ClientMaker: func(_ context.Context, clusterName string) (omnictrl.TalosClient, error) {
			fn, _ := m.Get(clusterName)

			return fn()
		},
		StoreFactory: factory,
		TickInterval: 10 * time.Minute,
	})
	require.NoError(t, err)
	require.NoError(t, testContext.Runtime.RegisterController(etcdBackupController))

	clustersData := createClusters(ctx, t, clusterNames, testContext.State, time.Hour)

	time.Sleep(11 * time.Minute)
	synctest.Wait()

	// Backups should be created for both clusters since those backups do not exist yet
	for _, cluster := range clustersData {
		findBackups(ctx, t, testContext.State, factory, cluster.Metadata().ID(), 1)
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
		testContext.State,
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

	findBackups(ctx, t, testContext.State, factory, clustersData[0].Metadata().ID(), 1)
	findBackups(ctx, t, testContext.State, factory, clustersData[1].Metadata().ID(), 2)

	for i := range clusterNames {
		rtestutils.Destroy[*omni.Cluster](ctx, t, testContext.State, []string{clustersData[i].Metadata().ID()})
	}

	assert.Equal(t, 5, talosClient.getNumSnapshots())
}

func TestDecryptEtcdBackup(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		testutils.WithRuntime(
			t.Context(),
			t,
			testutils.TestOptions{
				DisableCache: true,
			},
			func(_ context.Context, testContext testutils.TestContext) { // prepare - register controllers
				beforeStart(testContext.State, t, testContext.Runtime)
			},
			func(ctx context.Context, testContext testutils.TestContext) {
				testDecryptEtcdBackup(ctx, t, testContext)
			},
		)
	})
}

func testDecryptEtcdBackup(ctx context.Context, t *testing.T, testContext testutils.TestContext) {
	factory := startFactory(ctx, t, store.NewFileStoreStoreFactory(t.TempDir()), testContext.State, testContext.Logger)
	talosClient := &mockTalosClient{}

	etcdBackupController, err := omnictrl.NewEtcdBackupController(omnictrl.EtcdBackupControllerSettings{
		ClientMaker: func(context.Context, string) (omnictrl.TalosClient, error) {
			return talosClient, nil
		},
		StoreFactory: factory,
		TickInterval: 10 * time.Minute,
	})
	require.NoError(t, err)
	require.NoError(t, testContext.Runtime.RegisterController(etcdBackupController))

	clusterNames := []string{"talos-default-6"}
	clusters := createClusters(ctx, t, clusterNames, testContext.State, time.Hour)

	time.Sleep(11 * time.Minute)
	synctest.Wait()

	clusterBackups := findBackups(ctx, t, testContext.State, factory, clusters[0].Metadata().ID(), 1)

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
		rtestutils.Destroy[*omni.Cluster](ctx, t, testContext.State, []string{clusters[i].Metadata().ID()})
	}
}

func TestSingleListCall(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		testutils.WithRuntime(
			t.Context(),
			t,
			testutils.TestOptions{
				DisableCache: true,
			},
			func(_ context.Context, testContext testutils.TestContext) { // prepare - register controllers
				beforeStart(testContext.State, t, testContext.Runtime)
			},
			func(ctx context.Context, testContext testutils.TestContext) {
				testSingleListCall(ctx, t, testContext.Runtime, testContext.State)
			},
		)
	})
}

func testSingleListCall(ctx context.Context, t *testing.T, rt *runtime.Runtime, st state.State) {
	talosClient := &mockTalosClient{}
	store := &mockEtcdBackupStore{}
	storeFactory := &mockStoreFactory{
		store: store,
	}

	etcdBackupController, err := omnictrl.NewEtcdBackupController(omnictrl.EtcdBackupControllerSettings{
		ClientMaker: func(context.Context, string) (omnictrl.TalosClient, error) {
			return talosClient, nil
		},
		StoreFactory: storeFactory,
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

	assert.Len(t, store.getListCalls(), 2)
	assert.Len(t, store.getBackups(), 3)
}

func TestListBackupsWithExistingData(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		testutils.WithRuntime(
			t.Context(),
			t,
			testutils.TestOptions{
				DisableCache: true,
			},
			func(_ context.Context, testContext testutils.TestContext) { // prepare - register controllers
				beforeStart(testContext.State, t, testContext.Runtime)
			},
			func(ctx context.Context, testContext testutils.TestContext) {
				testListBackupsWithExistingData(ctx, t, testContext)
			},
		)
	})
}

func testListBackupsWithExistingData(ctx context.Context, t *testing.T, testContext testutils.TestContext) {
	clusterNames := []string{"talos-default-9", "talos-default-10"}
	clusters := createClusters(ctx, t, clusterNames, testContext.State, time.Hour)

	talosClient := &mockTalosClient{}
	store := &mockEtcdBackupStore{logger: testContext.Logger}

	// prepare the existing backup data
	for _, clusterBackupData := range clusters {
		uploadBackupData(ctx, t, store, clusterBackupData, time.Now())
		uploadBackupData(ctx, t, store, clusterBackupData, time.Now().Add(time.Minute))
	}

	sfm := &mockStoreFactory{store: store}

	etcdBackupController, err := omnictrl.NewEtcdBackupController(omnictrl.EtcdBackupControllerSettings{
		ClientMaker: func(context.Context, string) (omnictrl.TalosClient, error) {
			return talosClient, nil
		},
		StoreFactory: sfm,
		TickInterval: 10 * time.Minute,
	})
	require.NoError(t, err)
	require.NoError(t, testContext.Runtime.RegisterController(etcdBackupController))

	time.Sleep(11 * time.Minute)
	synctest.Wait()

	rtestutils.Destroy[*omni.Cluster](ctx, t, testContext.State, []string{clusters[0].Metadata().ID()})

	time.Sleep(time.Hour)
	synctest.Wait()
	time.Sleep(time.Hour)
	synctest.Wait()

	rtestutils.Destroy[*omni.Cluster](ctx, t, testContext.State, []string{clusters[1].Metadata().ID()})

	assert.Len(t, store.getListCalls(), 2)
	assert.Len(t, store.getBackups(), 6) // 4 initial backups + 2 2 backups during the test
}

func uploadBackupData(ctx context.Context, t *testing.T, store etcdbackup.Store, cluster *omni.BackupData, timestamp time.Time) {
	require.NoError(t,
		store.Upload(ctx, etcdbackup.Description{
			Timestamp:   timestamp,
			ClusterUUID: cluster.TypedSpec().Value.ClusterUuid,
			ClusterName: cluster.Metadata().ID(),
			EncryptionData: etcdbackup.EncryptionData{
				AESCBCEncryptionSecret:    cluster.TypedSpec().Value.AesCbcEncryptionSecret,
				SecretboxEncryptionSecret: cluster.TypedSpec().Value.SecretboxEncryptionSecret,
				EncryptionKey:             cluster.TypedSpec().Value.EncryptionKey,
			},
		}, strings.NewReader("mock data")),
	)
}

func TestEtcdManualBackupFindResource(t *testing.T) {
	t.Parallel()

	factory := store.NewFileStoreStoreFactory(t.TempDir())
	stateBuilder := testutils.DynamicStateBuilder{M: map[resource.Namespace]state.CoreState{}}

	stateBuilder.Set(resources.ExternalNamespace, &external.State{
		CoreState:    state.WrapCore(namespaced.NewState(stateBuilder.Builder)),
		StoreFactory: factory,
	})

	synctest.Test(t, func(t *testing.T) {
		testutils.WithRuntime(
			t.Context(),
			t,
			testutils.TestOptions{
				DisableCache: true,
				StateBuilder: stateBuilder.Builder,
			},
			func(_ context.Context, testContext testutils.TestContext) { // prepare - register controllers
				beforeStart(testContext.State, t, testContext.Runtime)
			},
			func(ctx context.Context, testContext testutils.TestContext) {
				testEtcdManualBackupFindResource(ctx, t, testContext, factory)
			},
		)
	})
}

func testEtcdManualBackupFindResource(ctx context.Context, t *testing.T, testContext testutils.TestContext, factory store.Factory) {
	factory = startFactory(ctx, t, factory, testContext.State, testContext.Logger)
	talosClient := &mockTalosClient{}

	etcdBackupController, err := omnictrl.NewEtcdBackupController(omnictrl.EtcdBackupControllerSettings{
		ClientMaker: func(context.Context, string) (omnictrl.TalosClient, error) {
			return talosClient, nil
		},
		StoreFactory: factory,
		TickInterval: time.Minute,
	})
	require.NoError(t, err)
	require.NoError(t, testContext.Runtime.RegisterController(etcdBackupController))

	clusterNames := []string{"talos-default-11"}
	clustersData := createClusters(ctx, t, clusterNames, testContext.State, 0) // 0 means that automatic backups are disabled

	manualBackup := omni.NewEtcdManualBackup(clustersData[0].Metadata().ID())
	manualBackup.TypedSpec().Value.BackupAt = timestamppb.New(time.Now().Add(15 * time.Second))

	err = testContext.State.Create(ctx, manualBackup)
	require.NoError(t, err)

	time.Sleep(15 * time.Second)
	synctest.Wait()

	backups := findBackups(ctx, t, testContext.State, factory, clustersData[0].Metadata().ID(), 1)

	// Should find backups by cluster ID

	backupRes := must.Value(safe.StateListAll[*omni.EtcdBackup](
		ctx,
		testContext.State,
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

	res := must.Value(safe.StateGetByID[*omni.EtcdBackup](ctx, testContext.State, backupRes.Get(0).Metadata().ID()))(t)

	assert.Equal(t, backups[0].Snapshot, res.TypedSpec().Value.Snapshot)
	assert.Equal(t, backups[0].Timestamp.UTC(), res.TypedSpec().Value.CreatedAt.AsTime())

	// Should return an error if no query is provided

	_, err = safe.StateList[*omni.EtcdBackup](ctx, testContext.State, omni.NewEtcdBackup("", time.Now()).Metadata())
	require.Error(t, err)

	for _, clusterData := range clustersData {
		rtestutils.Destroy[*omni.Cluster](ctx, t, testContext.State, []string{clusterData.Metadata().ID()})
	}
}

func TestEtcdManualBackup(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		testutils.WithRuntime(
			t.Context(),
			t,
			testutils.TestOptions{
				DisableCache: true,
			},
			func(_ context.Context, testContext testutils.TestContext) { // prepare - register controllers
				beforeStart(testContext.State, t, testContext.Runtime)
			},
			func(ctx context.Context, testContext testutils.TestContext) {
				testEtcdManualBackup(ctx, t, testContext)
			},
		)
	})
}

func testEtcdManualBackup(ctx context.Context, t *testing.T, testContext testutils.TestContext) {
	factory := startFactory(ctx, t, store.NewFileStoreStoreFactory(t.TempDir()), testContext.State, testContext.Logger)
	talosClient := &mockTalosClient{}
	st := testContext.State
	rt := testContext.Runtime

	etcdBackupController, err := omnictrl.NewEtcdBackupController(omnictrl.EtcdBackupControllerSettings{
		ClientMaker: func(context.Context, string) (omnictrl.TalosClient, error) {
			return talosClient, nil
		},
		StoreFactory: factory,
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
	findBackups(ctx, t, st, factory, clustersData[0].Metadata().ID(), 0)

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

	findBackups(ctx, t, st, factory, clustersData[0].Metadata().ID(), 1)

	time.Sleep(10 * time.Minute)
	synctest.Wait()

	// Should ignore this backup
	_, err = safe.StateUpdateWithConflicts(ctx, st, manualBackup.Metadata(), func(b *omni.EtcdManualBackup) error {
		b.TypedSpec().Value.BackupAt = timestamppb.New(time.Now().Add(-12 * time.Minute))

		return nil
	})
	require.NoError(t, err)

	findBackups(ctx, t, st, factory, clustersData[0].Metadata().ID(), 1)

	for _, clusterData := range clustersData {
		rtestutils.Destroy[*omni.Cluster](ctx, t, st, []string{clusterData.Metadata().ID()})
	}
}

func TestS3Backup(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		testutils.WithRuntime(
			t.Context(),
			t,
			testutils.TestOptions{
				DisableCache: true,
			},
			func(_ context.Context, testContext testutils.TestContext) { // prepare - register controllers
				beforeStart(testContext.State, t, testContext.Runtime)
			},
			func(ctx context.Context, testContext testutils.TestContext) {
				testS3Backup(ctx, t, testContext.Runtime, testContext.State)
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

	factory := startFactory(ctx, t, store.NewS3StoreFactory(), st, logger)
	talosClient := &mockTalosClient{}
	now := time.Now()

	etcdBackupController, err := omnictrl.NewEtcdBackupController(omnictrl.EtcdBackupControllerSettings{
		ClientMaker: func(context.Context, string) (omnictrl.TalosClient, error) {
			return talosClient, nil
		},
		StoreFactory: factory,
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

	it, err := must.Value(factory.GetStore())(t).ListBackups(ctx, clusters[0].TypedSpec().Value.ClusterUuid)
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

	synctest.Test(t, func(t *testing.T) {
		testutils.WithRuntime(
			t.Context(),
			t,
			testutils.TestOptions{
				DisableCache: true,
			},
			func(_ context.Context, testContext testutils.TestContext) { // prepare - register controllers
				beforeStart(testContext.State, t, testContext.Runtime)
			},
			func(ctx context.Context, testContext testutils.TestContext) {
				testBackupJitter(ctx, t, testContext)
			},
		)
	})
}

func testBackupJitter(ctx context.Context, t *testing.T, testContext testutils.TestContext) {
	factory := startFactory(ctx, t, store.NewFileStoreStoreFactory(t.TempDir()), testContext.State, testContext.Logger)
	talosClient := &mockTalosClient{}

	jitter := 20 * time.Minute
	etcdBackupController, err := omnictrl.NewEtcdBackupController(omnictrl.EtcdBackupControllerSettings{
		ClientMaker: func(context.Context, string) (omnictrl.TalosClient, error) {
			return talosClient, nil
		},
		StoreFactory: factory,
		TickInterval: 1 * time.Minute, // smaller tick, more chances for jitter to be applied
		Jitter:       jitter,
	})
	require.NoError(t, err)
	require.NoError(t, testContext.Runtime.RegisterController(etcdBackupController))

	clusterNames := []string{"talos-default-1", "talos-default-2"}
	clustersData := createClusters(ctx, t, clusterNames, testContext.State, time.Hour)

	synctest.Wait()                        // Wait for the first backup to be taken, it will be without jitter
	time.Sleep(time.Hour + 10*time.Minute) // Sleep until the next backup is due

	now := time.Now().UTC()

	synctest.Wait() // Wait for the second backup to be taken, this one will have jitter

	for _, cd := range clustersData {
		st := must.Value(factory.GetStore())(t)
		backups := must.Value(st.ListBackups(ctx, cd.TypedSpec().Value.ClusterUuid))(t)
		slc := toSlice(t, backups)

		require.Len(t, slc, 2)

		for _, b := range slc[:1] { // we are only interested in the last backup (reverse order)
			require.NotEqual(t, now, b.Timestamp.UTC())
			require.WithinDuration(t, now, b.Timestamp, jitter+30*time.Second)
		}
	}

	// Destroy the clusters
	for _, clusterData := range clustersData {
		rtestutils.Destroy[*omni.Cluster](ctx, t, testContext.State, []string{clusterData.Metadata().ID()})
	}

	assert.Equal(t, 4, talosClient.getNumSnapshots())
}

func createClusters(ctx context.Context, t *testing.T, clusterNames []string, st state.State, backupInterval time.Duration) []*omni.BackupData {
	clustersData := make([]*omni.BackupData, 0, len(clusterNames))

	for _, name := range clusterNames {
		secrets := omni.NewClusterSecrets(name)
		secrets.TypedSpec().Value.Data = secretsJSON

		require.NoError(t, st.Create(ctx, secrets))

		cluster := omni.NewCluster(name)
		cluster.TypedSpec().Value.TalosVersion = "1.3.0"
		cluster.TypedSpec().Value.BackupConfiguration = &specs.EtcdBackupConf{Interval: durationpb.New(backupInterval), Enabled: true}

		require.NoError(t, st.Create(ctx, cluster))
		require.NoError(t, st.Create(ctx, omni.NewMachineSet(omni.ControlPlanesResourceID(name))))

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

type mockTalosClient struct {
	callCount atomic.Uint32
}

func (m *mockTalosClient) getNumSnapshots() int {
	return int(m.callCount.Load())
}

func (m *mockTalosClient) EtcdSnapshot(context.Context, *machine.EtcdSnapshotRequest, ...grpc.CallOption) (io.ReadCloser, error) {
	m.callCount.Add(1)

	return io.NopCloser(strings.NewReader("Hello World")), nil
}

func startFactory(ctx context.Context, t *testing.T, factory store.Factory, st state.State, logger *zap.Logger) store.Factory {
	ctx, cancel := context.WithCancel(ctx)

	var eg errgroup.Group

	t.Cleanup(func() {
		cancel()
		require.NoError(t, eg.Wait())
	})

	eg.Go(func() error {
		return factory.Start(ctx, st, logger)
	})

	rtestutils.AssertResource(ctx, t, st, omni.EtcdBackupStoreStatusID, func(res *omni.EtcdBackupStoreStatus, assertion *assert.Assertions) {
		assertion.Empty(res.TypedSpec().Value.ConfigurationError)
	})

	return factory
}
