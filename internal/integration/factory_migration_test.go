// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build integration

package integration_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/extensions"
)

const (
	factoryMigrationStableCluster  = "integration-factory-migration-stable"
	factoryMigrationCurrentCluster = "integration-factory-migration-current"

	// annotationFactoryURL holds the factory URL of the current Talos version at the time of the snapshot.
	annotationFactoryURL = "factory-url"
)

// testFactoryMigrationPrepare runs against Omni configured with the public image factory as the primary one.
func testFactoryMigrationPrepare(options *TestOptions) TestFunc {
	return func(t *testing.T) {
		t.Log(`
Test the migration from the public image factory to the enterprise one, the first part that runs with the public factory as the primary
- create a cluster on the stable Talos version and a single node cluster, upgraded to the current Talos version right away
- check that no machine runs Talos Enterprise
- save the cluster snapshots for the later parts`)

		t.Parallel()

		options.claimMachines(t, 5)

		stableMachineOptions := MachineOptions{
			TalosVersion:      options.StableTalosVersion,
			KubernetesVersion: options.AnotherKubernetesVersion,
		}

		t.Run("StableClusterShouldBeCreated", CreateCluster(t.Context(), options, ClusterOptions{
			Name:                       factoryMigrationStableCluster,
			ControlPlanes:              3,
			Workers:                    1,
			MachineOptions:             stableMachineOptions,
			ScalingTimeout:             options.ScalingTimeout,
			SkipExtensionCheckOnCreate: options.SkipExtensionsCheckOnCreate,
		}))

		// machines booted from the stable version media can only join a cluster on that version, the second cluster is upgraded afterwards
		t.Run("CurrentClusterShouldBeCreated", CreateCluster(t.Context(), options, ClusterOptions{
			Name:                           factoryMigrationCurrentCluster,
			ControlPlanes:                  1,
			Workers:                        0,
			MachineOptions:                 stableMachineOptions,
			ScalingTimeout:                 options.ScalingTimeout,
			SkipExtensionCheckOnCreate:     options.SkipExtensionsCheckOnCreate,
			AllowSchedulingOnControlPlanes: true,
		}))

		assertClusterAndAPIReady(t, factoryMigrationStableCluster, options,
			withTalosVersion(stableMachineOptions.TalosVersion), withKubernetesVersion(stableMachineOptions.KubernetesVersion))
		assertClusterAndAPIReady(t, factoryMigrationCurrentCluster, options,
			withTalosVersion(stableMachineOptions.TalosVersion), withKubernetesVersion(stableMachineOptions.KubernetesVersion))

		t.Run("CurrentClusterShouldBeUpgraded", UpdateTalosVersion(t.Context(), options, factoryMigrationCurrentCluster, options.MachineOptions.TalosVersion))
		// a single node cluster has no Kubernetes API while its node reboots into the new version
		t.Run("CurrentClusterKubernetesAPIShouldBeAccessible", AssertKubernetesAPIAccessViaOmni(t.Context(), options.omniClient, factoryMigrationCurrentCluster, false, 5*time.Minute))
		t.Run("CurrentClusterBootstrapManifestSyncShouldBeSuccessful", KubernetesBootstrapManifestSync(t.Context(), options.omniClient.Management(), factoryMigrationCurrentCluster))

		assertClusterAndAPIReady(t, factoryMigrationCurrentCluster, options,
			withTalosVersion(options.MachineOptions.TalosVersion), withKubernetesVersion(stableMachineOptions.KubernetesVersion))

		t.Run("StableClusterShouldNotRunEnterprise", AssertMachinesRunEnterprise(t.Context(), options, factoryMigrationStableCluster, false))
		t.Run("CurrentClusterShouldNotRunEnterprise", AssertMachinesRunEnterprise(t.Context(), options, factoryMigrationCurrentCluster, false))

		t.Run("SaveStableClusterSnapshot", SaveClusterSnapshot(t.Context(), options, factoryMigrationStableCluster))
		t.Run("SaveCurrentClusterSnapshot", SaveClusterSnapshot(t.Context(), options, factoryMigrationCurrentCluster))
		t.Run("SaveFactoryURL", SaveTalosVersionFactoryURL(t.Context(), options, factoryMigrationStableCluster))
	}
}

// testFactoryMigrationVerify runs against Omni configured with the enterprise image factory as the primary one and the public one as the secondary.
func testFactoryMigrationVerify(options *TestOptions) TestFunc {
	return func(t *testing.T) {
		t.Log(`
Test the migration from the public image factory to the enterprise one, the second part that runs with the enterprise factory as the primary
- wait until the current Talos version resolves to the new factory
- check that the switch changed the machine configs (registry credentials) but rebooted nothing
- check that no machine runs Talos Enterprise yet
- upgrade the stable cluster to the current Talos version and check that all its machines run Talos Enterprise
- check that the cluster on the current version was left alone`)

		t.Parallel()

		stableMachineOptions := MachineOptions{
			TalosVersion:      options.StableTalosVersion,
			KubernetesVersion: options.AnotherKubernetesVersion,
		}

		t.Run("FactoryShouldChange", AssertTalosVersionFactoryURLChanged(t.Context(), options, factoryMigrationStableCluster))

		assertClusterAndAPIReady(t, factoryMigrationStableCluster, options,
			withTalosVersion(stableMachineOptions.TalosVersion), withKubernetesVersion(stableMachineOptions.KubernetesVersion))
		assertClusterAndAPIReady(t, factoryMigrationCurrentCluster, options, withKubernetesVersion(options.AnotherKubernetesVersion))

		t.Run("StableClusterConfigShouldChangeWithoutReboot", AssertClusterSnapshot(t.Context(), options, factoryMigrationStableCluster, withConfigChanged()))
		t.Run("CurrentClusterConfigShouldChangeWithoutReboot", AssertClusterSnapshot(t.Context(), options, factoryMigrationCurrentCluster, withConfigChanged()))

		t.Run("StableClusterShouldHaveNoPendingUpdates", AssertNoPendingMachineUpdates(t.Context(), options, factoryMigrationStableCluster))
		t.Run("CurrentClusterShouldHaveNoPendingUpdates", AssertNoPendingMachineUpdates(t.Context(), options, factoryMigrationCurrentCluster))

		// the credentials are in, from here on nothing on the current cluster is expected to change anymore
		t.Run("SaveCurrentClusterSnapshot", SaveClusterSnapshot(t.Context(), options, factoryMigrationCurrentCluster))

		t.Run("StableClusterShouldNotRunEnterprise", AssertMachinesRunEnterprise(t.Context(), options, factoryMigrationStableCluster, false))
		t.Run("CurrentClusterShouldNotRunEnterprise", AssertMachinesRunEnterprise(t.Context(), options, factoryMigrationCurrentCluster, false))

		t.Run("StableClusterShouldBeUpgraded", UpdateTalosVersion(t.Context(), options, factoryMigrationStableCluster, options.MachineOptions.TalosVersion))
		t.Run("StableClusterBootstrapManifestSyncShouldBeSuccessful", KubernetesBootstrapManifestSync(t.Context(), options.omniClient.Management(), factoryMigrationStableCluster))

		assertClusterAndAPIReady(t, factoryMigrationStableCluster, options,
			withTalosVersion(options.MachineOptions.TalosVersion), withKubernetesVersion(stableMachineOptions.KubernetesVersion))

		t.Run("StableClusterShouldRunEnterprise", AssertMachinesRunEnterprise(t.Context(), options, factoryMigrationStableCluster, true))
		t.Run("CurrentClusterShouldStillNotRunEnterprise", AssertMachinesRunEnterprise(t.Context(), options, factoryMigrationCurrentCluster, false))
		t.Run("CurrentClusterShouldNotChangeWhileStableUpgrades", AssertClusterSnapshot(t.Context(), options, factoryMigrationCurrentCluster))
		t.Run("CurrentClusterShouldHaveNoPendingUpdatesAfterUpgrade", AssertNoPendingMachineUpdates(t.Context(), options, factoryMigrationCurrentCluster))

		t.Run("SaveStableClusterSnapshot", SaveClusterSnapshot(t.Context(), options, factoryMigrationStableCluster))
		t.Run("SaveFactoryURL", SaveTalosVersionFactoryURL(t.Context(), options, factoryMigrationStableCluster))
	}
}

// testFactoryMigrationRollback runs against Omni configured with the public image factory as the primary one again and the enterprise one as the secondary.
func testFactoryMigrationRollback(options *TestOptions) TestFunc {
	return func(t *testing.T) {
		t.Log(`
Test the migration back from the enterprise image factory to the public one
- wait until the current Talos version resolves to the public factory again
- check that nothing changed and nothing rebooted: the machines keep the schematics they run, whichever factory issued them
- change the extensions of the cluster running Talos Enterprise and check that its machines run the public Talos again`)

		t.Parallel()

		// the stable cluster runs the current Talos version since the second part
		machineOptions := MachineOptions{
			TalosVersion:      options.MachineOptions.TalosVersion,
			KubernetesVersion: options.AnotherKubernetesVersion,
		}

		t.Run("FactoryShouldChange", AssertTalosVersionFactoryURLChanged(t.Context(), options, factoryMigrationStableCluster))

		assertClusterAndAPIReady(t, factoryMigrationStableCluster, options,
			withTalosVersion(machineOptions.TalosVersion), withKubernetesVersion(machineOptions.KubernetesVersion))
		assertClusterAndAPIReady(t, factoryMigrationCurrentCluster, options, withKubernetesVersion(options.AnotherKubernetesVersion))

		t.Run("StableClusterShouldHaveNoPendingUpdates", AssertNoPendingMachineUpdates(t.Context(), options, factoryMigrationStableCluster))
		t.Run("CurrentClusterShouldHaveNoPendingUpdates", AssertNoPendingMachineUpdates(t.Context(), options, factoryMigrationCurrentCluster))

		// nothing is expected to happen after the switch back, so there is no event to wait for, watch for a while instead
		t.Run("StableClusterShouldNotChange", AssertClusterSnapshotHolds(t.Context(), options, factoryMigrationStableCluster, 2*time.Minute))
		t.Run("CurrentClusterShouldNotChange", AssertClusterSnapshot(t.Context(), options, factoryMigrationCurrentCluster))

		t.Run("StableClusterShouldStillRunEnterprise", AssertMachinesRunEnterprise(t.Context(), options, factoryMigrationStableCluster, true))
		t.Run("CurrentClusterShouldStillNotRunEnterprise", AssertMachinesRunEnterprise(t.Context(), options, factoryMigrationCurrentCluster, false))

		newExtensions := []string{HelloWorldServiceExtensionName, extensions.OfficialPrefix + "qemu-guest-agent"}

		t.Run("StableClusterExtensionsShouldBeUpdated", AssertTalosExtensionsUpdateFlow(t.Context(), options.omniClient, factoryMigrationStableCluster, newExtensions))

		assertClusterAndAPIReady(t, factoryMigrationStableCluster, options,
			withTalosVersion(machineOptions.TalosVersion), withKubernetesVersion(machineOptions.KubernetesVersion))

		t.Run("StableClusterExtensionsShouldBePresent", AssertExtensionsArePresent(t.Context(), options, factoryMigrationStableCluster, newExtensions))
		t.Run("StableClusterShouldNotRunEnterpriseAnymore", AssertMachinesRunEnterprise(t.Context(), options, factoryMigrationStableCluster, false))

		// the rollout above took minutes, anything the switch back was going to do to the other cluster has happened by now
		t.Run("CurrentClusterShouldStillNotChange", AssertClusterSnapshot(t.Context(), options, factoryMigrationCurrentCluster))
	}
}

// AssertMachinesRunEnterprise asserts that every machine of the cluster does or does not carry the enterprise label.
func AssertMachinesRunEnterprise(testCtx context.Context, options *TestOptions, clusterName string, enterprise bool) TestFunc {
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(testCtx, 2*time.Minute)
		defer cancel()

		st := options.omniClient.Omni().State()

		machineIDs := rtestutils.ResourceIDs[*omni.ClusterMachine](ctx, t, st, state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, clusterName)))
		require.NotEmpty(t, machineIDs)

		rtestutils.AssertResources(ctx, t, st, machineIDs, func(res *omni.MachineStatus, assertion *assert.Assertions) {
			_, ok := res.Metadata().Labels().Get(omni.LabelEnterprise)

			assertion.Equal(enterprise, ok, "machine %q enterprise label", res.Metadata().ID())
		})
	}
}

// UpdateTalosVersion sets the Talos version of the cluster and waits for the upgrade to finish.
func UpdateTalosVersion(testCtx context.Context, options *TestOptions, clusterName, talosVersion string) TestFunc {
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(testCtx, 30*time.Minute)
		defer cancel()

		st := options.omniClient.Omni().State()

		t.Logf("upgrading cluster %q to %q", clusterName, talosVersion)

		_, err := safe.StateUpdateWithConflicts(ctx, st, omni.NewCluster(clusterName).Metadata(), func(cluster *omni.Cluster) error {
			cluster.TypedSpec().Value.TalosVersion = talosVersion

			return nil
		})
		require.NoError(t, err)

		rtestutils.AssertResources(ctx, t, st, []resource.ID{clusterName}, func(res *omni.TalosUpgradeStatus, assertion *assert.Assertions) {
			assertion.Equal(talosVersion, res.TypedSpec().Value.LastUpgradeVersion, resourceDetails(res))
			assertion.Empty(res.TypedSpec().Value.Step, resourceDetails(res))
		})
	}
}

// SaveTalosVersionFactoryURL records the image factory URL the current Talos version resolves to as a cluster annotation.
func SaveTalosVersionFactoryURL(testCtx context.Context, options *TestOptions, clusterName string) TestFunc {
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(testCtx, time.Minute)
		defer cancel()

		st := options.omniClient.Omni().State()

		talosVersion, err := safe.ReaderGetByID[*omni.TalosVersion](ctx, st, strings.TrimLeft(options.MachineOptions.TalosVersion, "v"))
		require.NoError(t, err)

		_, err = safe.StateUpdateWithConflicts(ctx, st, omni.NewCluster(clusterName).Metadata(), func(res *omni.Cluster) error {
			res.Metadata().Annotations().Set(annotationFactoryURL, talosVersion.TypedSpec().Value.ImageFactoryUrl)

			return nil
		})
		require.NoError(t, err)
	}
}

// AssertTalosVersionFactoryURLChanged waits until the current Talos version resolves to a factory other than the recorded one.
//
// The Talos version resources survive an Omni restart, so this is the signal that the restarted Omni picked up its new factory configuration.
func AssertTalosVersionFactoryURLChanged(testCtx context.Context, options *TestOptions, clusterName string) TestFunc {
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(testCtx, 5*time.Minute)
		defer cancel()

		st := options.omniClient.Omni().State()

		cluster, err := safe.ReaderGetByID[*omni.Cluster](ctx, st, clusterName)
		require.NoError(t, err)

		recordedURL, ok := cluster.Metadata().Annotations().Get(annotationFactoryURL)
		require.True(t, ok, "cluster does not have the factory URL annotation")

		rtestutils.AssertResources(ctx, t, st, []resource.ID{strings.TrimLeft(options.MachineOptions.TalosVersion, "v")}, func(res *omni.TalosVersion, assertion *assert.Assertions) {
			assertion.NotEqual(recordedURL, res.TypedSpec().Value.ImageFactoryUrl, "the Talos version still resolves to the previous factory")
		})
	}
}
