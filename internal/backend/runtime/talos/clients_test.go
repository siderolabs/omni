// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package talos_test

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/talos/pkg/machinery/config/bundle"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/secrets"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils"
	"github.com/siderolabs/omni/internal/backend/runtime/talos"
)

func TestGetClientForCluster(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
	t.Cleanup(cancel)

	testutils.WithRuntime(ctx, t, testutils.TestOptions{}, func(ctx context.Context, testContext testutils.TestContext) {
		require.NoError(t, testContext.Runtime.RegisterQController(secrets.NewTalosConfigController(constants.CertificateValidityTime)))
	}, func(ctx context.Context, testContext testutils.TestContext) {
		clusterName := "omni"
		clientFactory := talos.NewClientFactory(testContext.State, testContext.Logger)

		_, err := clientFactory.GetForCluster(ctx, clusterName)
		require.True(t, talos.IsClientNotReadyError(err))

		configBundle, err := bundle.NewBundle(bundle.WithInputOptions(
			&bundle.InputOptions{
				ClusterName: clusterName,
				Endpoint:    "https://127.0.0.1:6443",
			}))
		require.NoError(t, err)

		talosconfig := omni.NewTalosConfig(clusterName)
		spec := talosconfig.TypedSpec().Value

		context := configBundle.TalosCfg.Contexts[configBundle.TalosCfg.Context]

		spec.Ca = context.CA
		spec.Crt = context.Crt
		spec.Key = context.Key

		clusterStatus := omni.NewClusterStatus(clusterName)
		clusterStatus.TypedSpec().Value.Available = true

		require.NoError(t, testContext.State.Create(ctx, clusterStatus), state.WithCreateOwner((&omnictrl.ClusterStatusController{}).Name()))

		require.NoError(t, testContext.State.Create(ctx, talosconfig))

		clusterEndpoint := omni.NewClusterEndpoint(clusterName)
		clusterEndpoint.TypedSpec().Value.ManagementAddresses = []string{"localhost"}
		require.NoError(t, testContext.State.Create(ctx, clusterEndpoint))

		c1, err := clientFactory.GetForCluster(ctx, clusterName)
		require.NoError(t, err)

		c2, err := clientFactory.GetForCluster(ctx, clusterName)
		require.NoError(t, err)

		assert.Same(t, c1, c2)
	})
}

func TestClientLifecycle(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
	t.Cleanup(cancel)

	testutils.WithRuntime(ctx, t, testutils.TestOptions{}, func(ctx context.Context, testContext testutils.TestContext) {
	}, func(ctx context.Context, testContext testutils.TestContext) {
		// Use a nop logger because runtime.AddCleanup finalizers may run after the test completes,
		// and zaptest loggers panic when logging to a finished testing.T.
		clientFactory := talos.NewClientFactory(testContext.State, zap.NewNop())

		const clusterName = "alpha"

		// --- Setup: cluster credentials ---

		configBundle, err := bundle.NewBundle(bundle.WithInputOptions(
			&bundle.InputOptions{
				ClusterName: clusterName,
				Endpoint:    "https://127.0.0.1:6443",
			}))
		require.NoError(t, err)

		talosconfig := omni.NewTalosConfig(clusterName)
		bundleCtx := configBundle.TalosCfg.Contexts[configBundle.TalosCfg.Context]
		talosconfig.TypedSpec().Value.Ca = bundleCtx.CA
		talosconfig.TypedSpec().Value.Crt = bundleCtx.Crt
		talosconfig.TypedSpec().Value.Key = bundleCtx.Key
		require.NoError(t, testContext.State.Create(ctx, talosconfig))

		clusterEndpoint := omni.NewClusterEndpoint(clusterName)
		clusterEndpoint.TypedSpec().Value.ManagementAddresses = []string{"localhost"}
		require.NoError(t, testContext.State.Create(ctx, clusterEndpoint))

		// --- Setup: 5 machines with MachineStatus ---

		for _, id := range []string{"m1", "m2", "m3", "m4", "m5"} {
			ms := omni.NewMachineStatus(id)
			ms.TypedSpec().Value.ManagementAddress = "127.0.0.1"
			require.NoError(t, testContext.State.Create(ctx, ms))
		}

		// --- Setup: m1-m3 in cluster, m4-m5 in maintenance ---

		for _, id := range []string{"m1", "m2", "m3"} {
			cm := omni.NewClusterMachine(id)
			cm.Metadata().Labels().Set(omni.LabelCluster, clusterName)
			require.NoError(t, testContext.State.Create(ctx, cm))
		}

		// Start cache manager in background.
		var eg errgroup.Group

		eg.Go(func() error {
			return clientFactory.StartCacheManager(ctx)
		})

		t.Cleanup(func() {
			require.NoError(t, eg.Wait())
		})

		// --- Phase 1: Initial client creation ---

		clusterClient, err := clientFactory.GetForCluster(ctx, clusterName)
		require.NoError(t, err)
		assert.Equal(t, clusterName, clusterClient.ClusterID())
		assert.Empty(t, clusterClient.MachineID())

		clientM1, err := clientFactory.GetForMachine(ctx, "m1")
		require.NoError(t, err)
		assert.Equal(t, clusterName, clientM1.ClusterID())
		assert.Equal(t, "m1", clientM1.MachineID())

		clientM2, err := clientFactory.GetForMachine(ctx, "m2")
		require.NoError(t, err)
		assert.Equal(t, clusterName, clientM2.ClusterID())
		assert.Equal(t, "m2", clientM2.MachineID())

		maintenanceM4, err := clientFactory.GetForMachine(ctx, "m4")
		require.NoError(t, err)
		assert.Empty(t, maintenanceM4.ClusterID())
		assert.Equal(t, "m4", maintenanceM4.MachineID())

		maintenanceM5, err := clientFactory.GetForMachine(ctx, "m5")
		require.NoError(t, err)
		assert.Empty(t, maintenanceM5.ClusterID())
		assert.Equal(t, "m5", maintenanceM5.MachineID())

		// --- Phase 2: Caching — same calls return same pointers ---

		c, err := clientFactory.GetForCluster(ctx, clusterName)
		require.NoError(t, err)
		assert.Same(t, clusterClient, c)

		c, err = clientFactory.GetForMachine(ctx, "m1")
		require.NoError(t, err)
		assert.Same(t, clientM1, c)

		c, err = clientFactory.GetForMachine(ctx, "m4")
		require.NoError(t, err)
		assert.Same(t, maintenanceM4, c)

		// --- Phase 3: Machine joins cluster (m4) — maintenance client evicted ---

		cm4 := omni.NewClusterMachine("m4")
		cm4.Metadata().Labels().Set(omni.LabelCluster, clusterName)
		require.NoError(t, testContext.State.Create(ctx, cm4))

		var newM4 *talos.Client

		require.EventuallyWithT(t, func(collect *assert.CollectT) {
			runtime.Gosched() // allow the cache manager goroutine to process eviction events

			client, clientErr := clientFactory.GetForMachine(ctx, "m4")
			assert.NoError(collect, clientErr)
			assert.NotSame(collect, maintenanceM4, client, "m4 maintenance client should have been evicted")
			assert.Equal(collect, clusterName, client.ClusterID(), "m4 should now be a regular client")

			newM4 = client
		}, time.Minute, 100*time.Millisecond)

		// m5 should still be cached (unaffected).
		c, err = clientFactory.GetForMachine(ctx, "m5")
		require.NoError(t, err)
		assert.Same(t, maintenanceM5, c)

		// --- Phase 4: Machine leaves cluster (m2) — client evicted ---

		require.NoError(t, testContext.State.Destroy(ctx, omni.NewClusterMachine("m2").Metadata()))

		var newM2 *talos.Client

		require.EventuallyWithT(t, func(collect *assert.CollectT) {
			runtime.Gosched() // allow the cache manager goroutine to process eviction events

			client, clientErr := clientFactory.GetForMachine(ctx, "m2")
			assert.NoError(collect, clientErr)
			assert.NotSame(collect, clientM2, client, "m2 client should have been evicted")
			assert.Empty(collect, client.ClusterID(), "m2 should now be a maintenance client")

			newM2 = client
		}, time.Minute, 100*time.Millisecond)

		// m1 should still be cached (unaffected).
		c, err = clientFactory.GetForMachine(ctx, "m1")
		require.NoError(t, err)
		assert.Same(t, clientM1, c)

		// --- Phase 5: Cluster endpoint changes — all cluster clients evicted ---

		currentEndpoint, err := safe.StateGet[*omni.ClusterEndpoint](ctx, testContext.State, omni.NewClusterEndpoint(clusterName).Metadata())
		require.NoError(t, err)

		currentEndpoint.TypedSpec().Value.ManagementAddresses = []string{"other-address"}
		require.NoError(t, testContext.State.Update(ctx, currentEndpoint))

		// Cluster-wide client evicted.
		require.EventuallyWithT(t, func(collect *assert.CollectT) {
			runtime.Gosched() // allow the cache manager goroutine to process eviction events

			client, clientErr := clientFactory.GetForCluster(ctx, clusterName)
			assert.NoError(collect, clientErr)
			assert.NotSame(collect, clusterClient, client, "cluster client should have been evicted")
		}, time.Minute, 100*time.Millisecond)

		// Per-node clients also evicted (they share the cluster prefix).
		c, err = clientFactory.GetForMachine(ctx, "m1")
		require.NoError(t, err)
		assert.NotSame(t, clientM1, c, "m1 should have been evicted with the cluster")

		c, err = clientFactory.GetForMachine(ctx, "m4")
		require.NoError(t, err)
		assert.NotSame(t, newM4, c, "m4 should have been evicted with the cluster")

		// Maintenance clients unaffected.
		c, err = clientFactory.GetForMachine(ctx, "m5")
		require.NoError(t, err)
		assert.Same(t, maintenanceM5, c, "m5 maintenance client should be unaffected")

		c, err = clientFactory.GetForMachine(ctx, "m2")
		require.NoError(t, err)
		assert.Same(t, newM2, c, "m2 (now maintenance) should be unaffected")
	})
}
