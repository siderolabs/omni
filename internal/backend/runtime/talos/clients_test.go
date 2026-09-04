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
	"github.com/siderolabs/talos/pkg/machinery/config/generate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	"golang.org/x/sync/errgroup"

	"github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/secrets"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils"
	"github.com/siderolabs/omni/internal/backend/runtime/talos"
	"github.com/siderolabs/omni/internal/pkg/testsecrets"
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

		secretsBundle, err := testsecrets.Bundle(nil)
		require.NoError(t, err)

		configBundle, err := bundle.NewBundle(bundle.WithInputOptions(
			&bundle.InputOptions{
				ClusterName: clusterName,
				Endpoint:    "https://127.0.0.1:6443",
				KubeVersion: "1.36.1",
				GenOptions:  []generate.Option{generate.WithSecretsBundle(secretsBundle)},
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

		require.NoError(t, testContext.State.Create(ctx, clusterStatus, state.WithCreateOwner(omnictrl.ClusterStatusControllerName)))

		require.NoError(t, testContext.State.Create(ctx, talosconfig))

		clusterEndpoint := omni.NewClusterEndpoint(clusterName)
		clusterEndpoint.TypedSpec().Value.ManagementAddresses = []string{"localhost"}
		require.NoError(t, testContext.State.Create(ctx, clusterEndpoint))

		c1, err := clientFactory.GetForCluster(ctx, clusterName)
		require.NoError(t, err)

		defer c1.Close() //nolint:errcheck

		c2, err := clientFactory.GetForCluster(ctx, clusterName)
		require.NoError(t, err)

		defer c2.Close() //nolint:errcheck

		// two leases on the same cached connection
		assert.Same(t, c1.Client, c2.Client)
		assert.Equal(t, 1, clientFactory.CacheLen())
	})
}

func TestGetMaintenance(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
	t.Cleanup(cancel)

	testutils.WithRuntime(ctx, t, testutils.TestOptions{}, func(context.Context, testutils.TestContext) {
	}, func(ctx context.Context, testContext testutils.TestContext) {
		clientFactory := talos.NewClientFactory(testContext.State, testContext.Logger)

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

		defer maint.Close() //nolint:errcheck

		assert.Empty(t, maint.ClusterID())
		assert.Equal(t, "m-maint", maint.MachineID())

		// Cached: a second call returns a lease on the same connection.
		maintAgain, err := clientFactory.GetMaintenance(ctx, "m-maint")
		require.NoError(t, err)

		defer maintAgain.Close() //nolint:errcheck

		assert.Same(t, maint.Client, maintAgain.Client)

		// GetForMachine shares the same maintenance cache entry.
		viaForMachine, err := clientFactory.GetForMachine(ctx, "m-maint")
		require.NoError(t, err)

		defer viaForMachine.Close() //nolint:errcheck

		assert.Same(t, maint.Client, viaForMachine.Client)

		// --- Allocated machine still in maintenance: maintenance client, despite the cluster being set ---

		allocMaint, err := clientFactory.GetMaintenance(ctx, "m-alloc-maint")
		require.NoError(t, err)

		defer allocMaint.Close() //nolint:errcheck

		assert.Empty(t, allocMaint.ClusterID())
		assert.Equal(t, "m-alloc-maint", allocMaint.MachineID())

		// GetForMachine also returns the maintenance client, not a cluster client.
		allocMaintViaForMachine, err := clientFactory.GetForMachine(ctx, "m-alloc-maint")
		require.NoError(t, err)

		defer allocMaintViaForMachine.Close() //nolint:errcheck

		assert.Same(t, allocMaint.Client, allocMaintViaForMachine.Client)

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

// getAndClose obtains a client for the machine and closes it. Only its connection is meant to be compared afterwards.
func getAndClose(ctx context.Context, t *testing.T, clientFactory *talos.ClientFactory, machineID string) *talos.Client {
	c, err := clientFactory.GetForMachine(ctx, machineID)
	require.NoError(t, err)
	require.NoError(t, c.Close())

	return c
}

func TestClientLifecycle(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
	t.Cleanup(cancel)

	testutils.WithRuntime(ctx, t, testutils.TestOptions{}, func(ctx context.Context, testContext testutils.TestContext) {
	}, func(ctx context.Context, testContext testutils.TestContext) {
		clientFactory := talos.NewClientFactory(testContext.State, testContext.Logger)

		const clusterName = "alpha"

		// --- Setup: cluster credentials ---

		secretsBundle, err := testsecrets.Bundle(nil)
		require.NoError(t, err)

		configBundle, err := bundle.NewBundle(bundle.WithInputOptions(
			&bundle.InputOptions{
				ClusterName: clusterName,
				Endpoint:    "https://127.0.0.1:6443",
				KubeVersion: "1.36.1",
				GenOptions:  []generate.Option{generate.WithSecretsBundle(secretsBundle)},
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
		require.NoError(t, clusterClient.Close())
		assert.Equal(t, clusterName, clusterClient.ClusterID())
		assert.Empty(t, clusterClient.MachineID())

		clientM1 := getAndClose(ctx, t, clientFactory, "m1")
		assert.Equal(t, clusterName, clientM1.ClusterID())
		assert.Equal(t, "m1", clientM1.MachineID())

		clientM2 := getAndClose(ctx, t, clientFactory, "m2")
		assert.Equal(t, clusterName, clientM2.ClusterID())
		assert.Equal(t, "m2", clientM2.MachineID())

		maintenanceM4 := getAndClose(ctx, t, clientFactory, "m4")
		assert.Empty(t, maintenanceM4.ClusterID())
		assert.Equal(t, "m4", maintenanceM4.MachineID())

		maintenanceM5 := getAndClose(ctx, t, clientFactory, "m5")
		assert.Empty(t, maintenanceM5.ClusterID())
		assert.Equal(t, "m5", maintenanceM5.MachineID())

		// --- Phase 2: Caching, the same calls return leases on the same connections ---

		c, err := clientFactory.GetForCluster(ctx, clusterName)
		require.NoError(t, err)
		require.NoError(t, c.Close())
		assert.Same(t, clusterClient.Client, c.Client)

		c = getAndClose(ctx, t, clientFactory, "m1")
		assert.Same(t, clientM1.Client, c.Client)

		c = getAndClose(ctx, t, clientFactory, "m4")
		assert.Same(t, maintenanceM4.Client, c.Client)

		// --- Phase 3: Machine leaves maintenance (m4), maintenance client evicted ---

		ms4, err := safe.StateGet[*omni.MachineStatus](ctx, testContext.State, omni.NewMachineStatus("m4").Metadata())
		require.NoError(t, err)

		ms4.TypedSpec().Value.Maintenance = false
		ms4.TypedSpec().Value.Cluster = clusterName
		require.NoError(t, testContext.State.Update(ctx, ms4))

		var newM4 *talos.Client

		require.EventuallyWithT(t, func(collect *assert.CollectT) {
			client, clientErr := clientFactory.GetForMachine(ctx, "m4")
			if !assert.NoError(collect, clientErr) {
				return
			}

			assert.NoError(collect, client.Close())
			assert.NotSame(collect, maintenanceM4.Client, client.Client, "m4 maintenance client should have been evicted")
			assert.Equal(collect, clusterName, client.ClusterID(), "m4 should now be a regular client")

			newM4 = client
		}, time.Minute, 100*time.Millisecond)

		// m5 should still be cached (unaffected).
		c = getAndClose(ctx, t, clientFactory, "m5")
		assert.Same(t, maintenanceM5.Client, c.Client)

		// --- Phase 4: Machine leaves cluster (m2), client evicted ---

		// The machine goes back to maintenance mode and its status clears the cluster.
		ms2, err := safe.StateGet[*omni.MachineStatus](ctx, testContext.State, omni.NewMachineStatus("m2").Metadata())
		require.NoError(t, err)

		ms2.TypedSpec().Value.Maintenance = true
		ms2.TypedSpec().Value.Cluster = ""
		require.NoError(t, testContext.State.Update(ctx, ms2))

		var newM2 *talos.Client

		require.EventuallyWithT(t, func(collect *assert.CollectT) {
			client, clientErr := clientFactory.GetForMachine(ctx, "m2")
			if !assert.NoError(collect, clientErr) {
				return
			}

			assert.NoError(collect, client.Close())
			assert.NotSame(collect, clientM2.Client, client.Client, "m2 client should have been evicted")
			assert.Empty(collect, client.ClusterID(), "m2 should now be a maintenance client")

			newM2 = client
		}, time.Minute, 100*time.Millisecond)

		// m1 should still be cached (unaffected).
		c = getAndClose(ctx, t, clientFactory, "m1")
		assert.Same(t, clientM1.Client, c.Client)

		// --- Phase 5: Cluster endpoint changes, all cluster clients evicted ---

		currentEndpoint, err := safe.StateGet[*omni.ClusterEndpoint](ctx, testContext.State, omni.NewClusterEndpoint(clusterName).Metadata())
		require.NoError(t, err)

		currentEndpoint.TypedSpec().Value.ManagementAddresses = []string{"other-address"}
		require.NoError(t, testContext.State.Update(ctx, currentEndpoint))

		// Cluster-wide client evicted.
		require.EventuallyWithT(t, func(collect *assert.CollectT) {
			client, clientErr := clientFactory.GetForCluster(ctx, clusterName)
			if !assert.NoError(collect, clientErr) {
				return
			}

			assert.NoError(collect, client.Close())
			assert.NotSame(collect, clusterClient.Client, client.Client, "cluster client should have been evicted")
		}, time.Minute, 100*time.Millisecond)

		// Per-node clients also evicted (they share the cluster prefix).
		c = getAndClose(ctx, t, clientFactory, "m1")
		assert.NotSame(t, clientM1.Client, c.Client, "m1 should have been evicted with the cluster")

		c = getAndClose(ctx, t, clientFactory, "m4")
		assert.NotSame(t, newM4.Client, c.Client, "m4 should have been evicted with the cluster")

		// Maintenance clients unaffected.
		c = getAndClose(ctx, t, clientFactory, "m5")
		assert.Same(t, maintenanceM5.Client, c.Client, "m5 maintenance client should be unaffected")

		c = getAndClose(ctx, t, clientFactory, "m2")
		assert.Same(t, newM2.Client, c.Client, "m2 (now maintenance) should be unaffected")
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
		clientFactory := talos.NewClientFactory(testContext.State, testContext.Logger)

		const clusterName = "alpha"

		// Cluster credentials, required to build a secure cluster client.
		secretsBundle, err := testsecrets.Bundle(nil)
		require.NoError(t, err)

		configBundle, err := bundle.NewBundle(bundle.WithInputOptions(
			&bundle.InputOptions{
				ClusterName: clusterName,
				Endpoint:    "https://127.0.0.1:6443",
				KubeVersion: "1.36.1",
				GenOptions:  []generate.Option{generate.WithSecretsBundle(secretsBundle)},
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
		clusterClient := getAndClose(ctx, t, clientFactory, "m1")
		require.Equal(t, clusterName, clusterClient.ClusterID())

		// --- Machine leaves the cluster and goes back to maintenance: cluster field is cleared in the same update ---

		ms, err = safe.StateGet[*omni.MachineStatus](ctx, testContext.State, omni.NewMachineStatus("m1").Metadata())
		require.NoError(t, err)

		ms.TypedSpec().Value.Maintenance = true
		ms.TypedSpec().Value.Cluster = ""
		require.NoError(t, testContext.State.Update(ctx, ms))

		require.EventuallyWithT(t, func(collect *assert.CollectT) {
			client, clientErr := clientFactory.GetForMachine(ctx, "m1")
			if !assert.NoError(collect, clientErr) {
				return
			}

			assert.NoError(collect, client.Close())
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
			if !assert.NoError(collect, clientErr) {
				return
			}

			assert.NoError(collect, client.Close())
			assert.Equal(collect, clusterName, client.ClusterID(), "m1 should be a cluster client again")
			assert.NotSame(collect, clusterClient.Client, client.Client, "stale cluster client must have been evicted on cluster leave")
		}, time.Minute, 100*time.Millisecond)
	})
}

// createClusterMachine creates the status of a machine allocated to the cluster and reachable over the given address.
//
// The address must be a unix socket, as a cluster client over any other address needs the cluster credentials.
func createClusterMachine(ctx context.Context, t *testing.T, st state.State, id, clusterName, address string) {
	ms := omni.NewMachineStatus(id)
	ms.TypedSpec().Value.ManagementAddress = address
	ms.TypedSpec().Value.Cluster = clusterName
	require.NoError(t, st.Create(ctx, ms))
}

// createMaintenanceMachine creates the status of a machine in maintenance mode. Its client needs no credentials.
func createMaintenanceMachine(ctx context.Context, t *testing.T, st state.State, id string) {
	ms := omni.NewMachineStatus(id)
	ms.TypedSpec().Value.ManagementAddress = "127.0.0.1"
	ms.TypedSpec().Value.Maintenance = true
	require.NoError(t, st.Create(ctx, ms))
}

// TestClientLease checks that the shared connection stays open as long as any client is open, cached or not, and is
// closed after the last one.
func TestClientLease(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	t.Cleanup(cancel)

	testutils.WithRuntime(ctx, t, testutils.TestOptions{}, func(context.Context, testutils.TestContext) {
	}, func(ctx context.Context, testContext testutils.TestContext) {
		clientFactory := talos.NewClientFactory(testContext.State, testContext.Logger)

		// the machine is reachable over the unix socket of the machine mock, so the clients can do real RPCs
		machineMock := testutils.NewMachineServiceMock(ctx, t, "m1", testContext.State)
		createClusterMachine(ctx, t, testContext.State, "m1", "alpha", machineMock.SocketConnectionString)

		c1, err := clientFactory.GetForMachine(ctx, "m1")
		require.NoError(t, err)

		c2, err := clientFactory.GetForMachine(ctx, "m1")
		require.NoError(t, err)

		assert.Same(t, c1.Client, c2.Client)
		assert.Equal(t, 1, clientFactory.CacheLen())
		assert.Equal(t, 1, clientFactory.ActiveClients("machine"))

		// evicted from the cache while both clients are open: the connection stays open
		clientFactory.ReleaseForMachine("alpha", "m1")

		assert.Equal(t, 0, clientFactory.CacheLen())
		assert.Equal(t, 1, clientFactory.ActiveClients("machine"))

		_, err = c1.Version(ctx)
		require.NoError(t, err)

		// closing one client keeps the connection open for the other one
		require.NoError(t, c1.Close())

		_, err = c2.Version(ctx)
		require.NoError(t, err)

		assert.Equal(t, 1, clientFactory.ActiveClients("machine"))

		// closing the last client closes the connection, closing it again is a no-op
		require.NoError(t, c2.Close())
		require.NoError(t, c2.Close())

		_, err = c2.Version(ctx)
		require.Error(t, err)

		assert.Equal(t, 0, clientFactory.ActiveClients("machine"))

		// a new client gets a new connection
		c3, err := clientFactory.GetForMachine(ctx, "m1")
		require.NoError(t, err)

		assert.NotSame(t, c1.Client, c3.Client)

		_, err = c3.Version(ctx)
		require.NoError(t, err)

		// closing the client before the eviction keeps the connection cached
		require.NoError(t, c3.Close())

		assert.Equal(t, 1, clientFactory.CacheLen())
		assert.Equal(t, 1, clientFactory.ActiveClients("machine"))

		c4, err := clientFactory.GetForMachine(ctx, "m1")
		require.NoError(t, err)

		assert.Same(t, c3.Client, c4.Client)

		_, err = c4.Version(ctx)
		require.NoError(t, err)

		// the eviction of a cached connection without open clients closes it
		require.NoError(t, c4.Close())

		clientFactory.ReleaseForMachine("alpha", "m1")

		assert.Equal(t, 0, clientFactory.CacheLen())
		assert.Equal(t, 0, clientFactory.ActiveClients("machine"))
	})
}

// TestClientSweep checks that the idle connections are closed after the idle timeout, and the ones with open clients are
// not counted as idle.
func TestClientSweep(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	t.Cleanup(cancel)

	testutils.WithRuntime(ctx, t, testutils.TestOptions{}, func(context.Context, testutils.TestContext) {
	}, func(ctx context.Context, testContext testutils.TestContext) {
		clientFactory := talos.NewClientFactory(testContext.State, testContext.Logger)

		createMaintenanceMachine(ctx, t, testContext.State, "m1")
		createMaintenanceMachine(ctx, t, testContext.State, "m2")

		getAndClose(ctx, t, clientFactory, "m1")

		c2, err := clientFactory.GetForMachine(ctx, "m2")
		require.NoError(t, err)

		assert.Equal(t, 2, clientFactory.CacheLen())

		// nothing is idle for long enough yet
		clientFactory.Sweep(time.Now())

		assert.Equal(t, 2, clientFactory.CacheLen())

		// m1 has no open clients and is idle for longer than the timeout, m2 has an open client
		clientFactory.Sweep(time.Now().Add(talos.IdleTimeout))

		assert.Equal(t, 1, clientFactory.CacheLen())
		assert.Equal(t, 1, clientFactory.ActiveClients("maintenance"))

		// the idle time of m2 starts when its last client is closed
		require.NoError(t, c2.Close())

		clientFactory.Sweep(time.Now())

		assert.Equal(t, 1, clientFactory.CacheLen())

		clientFactory.Sweep(time.Now().Add(talos.IdleTimeout))

		assert.Equal(t, 0, clientFactory.CacheLen())
		assert.Equal(t, 0, clientFactory.ActiveClients("maintenance"))
	})
}

// TestClientFactoryStop checks that stopping the factory evicts everything, keeps the open clients working and refuses
// to cache new clients.
func TestClientFactoryStop(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	t.Cleanup(cancel)

	testutils.WithRuntime(ctx, t, testutils.TestOptions{}, func(context.Context, testutils.TestContext) {
	}, func(ctx context.Context, testContext testutils.TestContext) {
		clientFactory := talos.NewClientFactory(testContext.State, testContext.Logger)

		machineMock := testutils.NewMachineServiceMock(ctx, t, "m1", testContext.State)
		createClusterMachine(ctx, t, testContext.State, "m1", "alpha", machineMock.SocketConnectionString)

		c, err := clientFactory.GetForMachine(ctx, "m1")
		require.NoError(t, err)

		clientFactory.Stop()

		assert.Equal(t, 0, clientFactory.CacheLen())

		_, err = c.Version(ctx)
		require.NoError(t, err)

		require.NoError(t, c.Close())

		assert.Equal(t, 0, clientFactory.ActiveClients("machine"))

		_, err = clientFactory.GetForMachine(ctx, "m1")
		require.ErrorContains(t, err, "stopped")
	})
}

// TestClientLeakGuard checks that a client garbage collected without being closed is reported, and a closed one is not.
func TestClientLeakGuard(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	t.Cleanup(cancel)

	testutils.WithRuntime(ctx, t, testutils.TestOptions{}, func(context.Context, testutils.TestContext) {
	}, func(ctx context.Context, testContext testutils.TestContext) {
		// the leak report can be logged after the test is over, so the logger must not be bound to the test
		core, logs := observer.New(zapcore.WarnLevel)
		clientFactory := talos.NewClientFactory(testContext.State, zap.New(core))

		createMaintenanceMachine(ctx, t, testContext.State, "m1")

		leaked := func() bool {
			return len(logs.FilterMessageSnippet("without being closed").All()) > 0
		}

		collectGarbage := func() {
			for range 3 {
				runtime.GC()
				time.Sleep(10 * time.Millisecond)
			}
		}

		// a closed client is not reported. it is obtained in a function, so no reference to it is left on the stack
		func() {
			getAndClose(ctx, t, clientFactory, "m1")
		}()

		collectGarbage()

		assert.False(t, leaked())

		// a client which is never closed is reported
		func() {
			_, err := clientFactory.GetForMachine(ctx, "m1")
			require.NoError(t, err)
		}()

		require.EventuallyWithT(t, func(collect *assert.CollectT) {
			collectGarbage()

			assert.True(collect, leaked())
		}, 10*time.Second, 50*time.Millisecond)

		// the entry is still cached and still counted as leased, so the sweep never drops it
		clientFactory.Sweep(time.Now().Add(talos.IdleTimeout))

		assert.Equal(t, 1, clientFactory.CacheLen())

		clientFactory.Stop()
	})
}

// TestClientFactoryConcurrency runs lookups, closes, evictions and sweeps concurrently and checks that nothing is left open.
func TestClientFactoryConcurrency(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	t.Cleanup(cancel)

	testutils.WithRuntime(ctx, t, testutils.TestOptions{}, func(context.Context, testutils.TestContext) {
	}, func(ctx context.Context, testContext testutils.TestContext) {
		clientFactory := talos.NewClientFactory(testContext.State, zap.NewNop())

		machines := []string{"m1", "m2", "m3"}

		for _, id := range machines {
			createMaintenanceMachine(ctx, t, testContext.State, id)
		}

		var eg errgroup.Group

		for range 8 {
			eg.Go(func() error {
				for i := range 200 {
					c, err := clientFactory.GetForMachine(ctx, machines[i%len(machines)])
					if err != nil {
						return err
					}

					if len(c.GetEndpoints()) == 0 {
						return context.Canceled
					}

					if err = c.Close(); err != nil {
						return err
					}
				}

				return nil
			})
		}

		eg.Go(func() error {
			for i := range 200 {
				clientFactory.ReleaseForMachine("", machines[i%len(machines)])
				clientFactory.Sweep(time.Now().Add(talos.IdleTimeout))
			}

			return nil
		})

		require.NoError(t, eg.Wait())

		clientFactory.Stop()

		assert.Equal(t, 0, clientFactory.CacheLen())
		assert.Equal(t, 0, clientFactory.ActiveClients("maintenance"))
	})
}

// TestRuntimeReleasesClientOnError checks that the runtime does not keep a client open when it fails to hand it out.
func TestRuntimeReleasesClientOnError(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	t.Cleanup(cancel)

	testutils.WithRuntime(ctx, t, testutils.TestOptions{}, func(context.Context, testutils.TestContext) {
	}, func(ctx context.Context, testContext testutils.TestContext) {
		clientFactory := talos.NewClientFactory(testContext.State, testContext.Logger)
		talosRuntime := talos.New(clientFactory, testContext.Logger, "omni", "https://omni.example.org")

		createMaintenanceMachine(ctx, t, testContext.State, "m1")

		// the machine has a status but no machine resource, so the connectivity check fails
		_, err := talosRuntime.GetClientForMachine(ctx, "m1")
		require.Error(t, err)

		// the client was released: the cached connection has no open clients, so the sweep drops it
		assert.Equal(t, 1, clientFactory.CacheLen())

		clientFactory.Sweep(time.Now().Add(talos.IdleTimeout))

		assert.Equal(t, 0, clientFactory.CacheLen())
		assert.Equal(t, 0, clientFactory.ActiveClients("maintenance"))

		// the machine is known but not connected, so the client is refused and released as well
		require.NoError(t, testContext.State.Create(ctx, omni.NewMachine("m1")))

		_, err = talosRuntime.GetClientForMachine(ctx, "m1")
		require.ErrorContains(t, err, "not reachable")

		clientFactory.Sweep(time.Now().Add(talos.IdleTimeout))

		assert.Equal(t, 0, clientFactory.CacheLen())
		assert.Equal(t, 0, clientFactory.ActiveClients("maintenance"))
	})
}
