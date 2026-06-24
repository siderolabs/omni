// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package talos_test

import (
	"context"
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
				KubeVersion: "1.36.1",
			},
		))
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

func TestGetMaintenance(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
	t.Cleanup(cancel)

	testutils.WithRuntime(ctx, t, testutils.TestOptions{}, func(context.Context, testutils.TestContext) {
	}, func(ctx context.Context, testContext testutils.TestContext) {
		// Use a nop logger because runtime.AddCleanup finalizers may run after the test completes,
		// and zaptest loggers panic when logging to a finished testing.T.
		clientFactory := talos.NewClientFactory(testContext.State, zap.NewNop())

		// A machine in maintenance mode: it has a status with the maintenance flag set.
		maintMachine := omni.NewMachineStatus("m-maint")
		maintMachine.TypedSpec().Value.ManagementAddress = "127.0.0.1"
		maintMachine.TypedSpec().Value.Maintenance = true
		require.NoError(t, testContext.State.Create(ctx, maintMachine))

		// A configured machine: it is allocated to a cluster and no longer in maintenance mode.
		configuredMachine := omni.NewMachineStatus("m-configured")
		configuredMachine.TypedSpec().Value.ManagementAddress = "127.0.0.1"
		configuredMachine.TypedSpec().Value.Cluster = "alpha"
		require.NoError(t, testContext.State.Create(ctx, configuredMachine))

		// An allocated machine that is still in maintenance mode: it has a cluster set, but its initial
		// configuration has not been applied yet, so it is only reachable over the maintenance connection.
		allocatedMaintMachine := omni.NewMachineStatus("m-alloc-maint")
		allocatedMaintMachine.TypedSpec().Value.ManagementAddress = "127.0.0.1"
		allocatedMaintMachine.TypedSpec().Value.Cluster = "alpha"
		allocatedMaintMachine.TypedSpec().Value.Maintenance = true
		require.NoError(t, testContext.State.Create(ctx, allocatedMaintMachine))

		// --- Maintenance machine: returns a maintenance (no cluster) client ---

		maint, err := clientFactory.GetMaintenance(ctx, "m-maint")
		require.NoError(t, err)
		assert.Empty(t, maint.ClusterID())
		assert.Equal(t, "m-maint", maint.MachineID())

		// Cached: a second call returns the same pointer.
		maintAgain, err := clientFactory.GetMaintenance(ctx, "m-maint")
		require.NoError(t, err)
		assert.Same(t, maint, maintAgain)

		// GetForMachine shares the same maintenance cache entry.
		viaForMachine, err := clientFactory.GetForMachine(ctx, "m-maint")
		require.NoError(t, err)
		assert.Same(t, maint, viaForMachine)

		// --- Allocated machine still in maintenance: maintenance client, despite the cluster being set ---

		allocMaint, err := clientFactory.GetMaintenance(ctx, "m-alloc-maint")
		require.NoError(t, err)
		assert.Empty(t, allocMaint.ClusterID())
		assert.Equal(t, "m-alloc-maint", allocMaint.MachineID())

		// GetForMachine also returns the maintenance client, not a cluster client.
		allocMaintViaForMachine, err := clientFactory.GetForMachine(ctx, "m-alloc-maint")
		require.NoError(t, err)
		assert.Same(t, allocMaint, allocMaintViaForMachine)

		// --- Configured machine: refuses to return a maintenance client ---

		_, err = clientFactory.GetMaintenance(ctx, "m-configured")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "is not in maintenance mode")
		assert.False(t, talos.IsClientNotReadyError(err))

		// --- Unknown machine (no status yet): not ready ---

		_, err = clientFactory.GetMaintenance(ctx, "m-unknown")
		require.True(t, talos.IsClientNotReadyError(err))
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
				KubeVersion: "1.36.1",
			},
		))
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

		// --- Setup: m1-m3 configured in cluster, m4-m5 in maintenance ---

		for _, id := range []string{"m1", "m2", "m3"} {
			ms := omni.NewMachineStatus(id)
			ms.TypedSpec().Value.ManagementAddress = "127.0.0.1"
			ms.TypedSpec().Value.Cluster = clusterName
			require.NoError(t, testContext.State.Create(ctx, ms))
		}

		for _, id := range []string{"m4", "m5"} {
			ms := omni.NewMachineStatus(id)
			ms.TypedSpec().Value.ManagementAddress = "127.0.0.1"
			ms.TypedSpec().Value.Maintenance = true
			require.NoError(t, testContext.State.Create(ctx, ms))
		}

		// Start cache manager in background.
		var eg errgroup.Group

		eg.Go(func() error {
			return clientFactory.StartCacheManager(ctx)
		})

		t.Cleanup(func() {
			require.NoError(t, eg.Wait())
		})

		// Wait until the cache manager has registered its watches before mutating state below, so its eviction events
		// are not missed.
		require.NoError(t, clientFactory.WaitForCacheStart(ctx))

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

		// --- Phase 3: Machine leaves maintenance (m4) — maintenance client evicted ---

		ms4, err := safe.StateGet[*omni.MachineStatus](ctx, testContext.State, omni.NewMachineStatus("m4").Metadata())
		require.NoError(t, err)

		ms4.TypedSpec().Value.Maintenance = false
		ms4.TypedSpec().Value.Cluster = clusterName
		require.NoError(t, testContext.State.Update(ctx, ms4))

		var newM4 *talos.Client

		require.EventuallyWithT(t, func(collect *assert.CollectT) {
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

		// The machine goes back to maintenance mode and its status clears the cluster.
		ms2, err := safe.StateGet[*omni.MachineStatus](ctx, testContext.State, omni.NewMachineStatus("m2").Metadata())
		require.NoError(t, err)

		ms2.TypedSpec().Value.Maintenance = true
		ms2.TypedSpec().Value.Cluster = ""
		require.NoError(t, testContext.State.Update(ctx, ms2))

		var newM2 *talos.Client

		require.EventuallyWithT(t, func(collect *assert.CollectT) {
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

// TestClusterClientEvictedOnClusterLeave verifies that the secure cluster client of a machine is evicted when the machine
// leaves its cluster, so that a later rejoin to the same cluster does not reuse the stale client.
//
// The machine status clears its cluster field as the machine leaves, so the cluster name needed to evict the secure
// cluster client is read from the previous version of the resource carried by the update event. This is why the cache
// manager does not need to watch cluster machines.
func TestClusterClientEvictedOnClusterLeave(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
	t.Cleanup(cancel)

	testutils.WithRuntime(ctx, t, testutils.TestOptions{}, func(context.Context, testutils.TestContext) {
	}, func(ctx context.Context, testContext testutils.TestContext) {
		// Use a nop logger because runtime.AddCleanup finalizers may run after the test completes,
		// and zaptest loggers panic when logging to a finished testing.T.
		clientFactory := talos.NewClientFactory(testContext.State, zap.NewNop())

		const clusterName = "alpha"

		// Cluster credentials, required to build a secure cluster client.
		configBundle, err := bundle.NewBundle(bundle.WithInputOptions(
			&bundle.InputOptions{
				ClusterName: clusterName,
				Endpoint:    "https://127.0.0.1:6443",
				KubeVersion: "1.36.1",
			},
		))
		require.NoError(t, err)

		talosconfig := omni.NewTalosConfig(clusterName)
		bundleCtx := configBundle.TalosCfg.Contexts[configBundle.TalosCfg.Context]
		talosconfig.TypedSpec().Value.Ca = bundleCtx.CA
		talosconfig.TypedSpec().Value.Crt = bundleCtx.Crt
		talosconfig.TypedSpec().Value.Key = bundleCtx.Key
		require.NoError(t, testContext.State.Create(ctx, talosconfig))

		// A machine configured in the cluster.
		ms := omni.NewMachineStatus("m1")
		ms.TypedSpec().Value.ManagementAddress = "127.0.0.1"
		ms.TypedSpec().Value.Cluster = clusterName
		require.NoError(t, testContext.State.Create(ctx, ms))

		var eg errgroup.Group

		eg.Go(func() error {
			return clientFactory.StartCacheManager(ctx)
		})

		t.Cleanup(func() {
			require.NoError(t, eg.Wait())
		})

		// Wait until the cache manager has registered its watches before mutating state. Otherwise the cluster leave
		// below can happen before the watches exist and its eviction event would be missed.
		require.NoError(t, clientFactory.WaitForCacheStart(ctx))

		// Initial secure cluster client.
		clusterClient, err := clientFactory.GetForMachine(ctx, "m1")
		require.NoError(t, err)
		require.Equal(t, clusterName, clusterClient.ClusterID())

		// --- Machine leaves the cluster and goes back to maintenance: cluster field is cleared in the same update ---

		ms, err = safe.StateGet[*omni.MachineStatus](ctx, testContext.State, omni.NewMachineStatus("m1").Metadata())
		require.NoError(t, err)

		ms.TypedSpec().Value.Maintenance = true
		ms.TypedSpec().Value.Cluster = ""
		require.NoError(t, testContext.State.Update(ctx, ms))

		require.EventuallyWithT(t, func(collect *assert.CollectT) {
			client, clientErr := clientFactory.GetForMachine(ctx, "m1")
			assert.NoError(collect, clientErr)
			assert.Empty(collect, client.ClusterID(), "m1 should now be a maintenance client")
		}, time.Minute, 100*time.Millisecond)

		// --- Machine rejoins the same cluster ---

		ms, err = safe.StateGet[*omni.MachineStatus](ctx, testContext.State, omni.NewMachineStatus("m1").Metadata())
		require.NoError(t, err)

		ms.TypedSpec().Value.Maintenance = false
		ms.TypedSpec().Value.Cluster = clusterName
		require.NoError(t, testContext.State.Update(ctx, ms))

		// The cluster client built before the machine left must not be reused: it was evicted on leave using the previous
		// cluster name from the update event. Otherwise the rejoined machine would be served the stale client.
		require.EventuallyWithT(t, func(collect *assert.CollectT) {
			client, clientErr := clientFactory.GetForMachine(ctx, "m1")
			assert.NoError(collect, clientErr)
			assert.Equal(collect, clusterName, client.ClusterID(), "m1 should be a cluster client again")
			assert.NotSame(collect, clusterClient, client, "stale cluster client must have been evicted on cluster leave")
		}, time.Minute, 100*time.Millisecond)
	})
}
