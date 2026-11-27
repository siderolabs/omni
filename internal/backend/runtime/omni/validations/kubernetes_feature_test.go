// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package validations_test

import (
	"context"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/siderolabs/omni/client/api/omni/specs"
	omnires "github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/etcdbackup/store"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/validated"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/validations"
	"github.com/siderolabs/omni/internal/pkg/config"
)

const (
	featureUnset    = specs.FeatureState_FEATURE_STATE_UNSPECIFIED
	featureEnabled  = specs.FeatureState_FEATURE_STATE_ENABLED
	featureDisabled = specs.FeatureState_FEATURE_STATE_DISABLED
)

func newKubernetesFeatureState(t *testing.T) (context.Context, state.State) {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	t.Cleanup(cancel)

	etcdBackupStoreFactory, err := store.NewStoreFactory(config.EtcdBackup{})
	require.NoError(t, err)

	innerSt := state.WrapCore(inmem.NewState())

	etcdBackupConfig := config.EtcdBackup{
		TickInterval: new(time.Minute),
		MinInterval:  new(time.Hour),
		MaxInterval:  new(24 * time.Hour),
	}

	opts := validations.ClusterValidationOptions(innerSt, etcdBackupConfig, config.EmbeddedDiscoveryService{})
	opts = append(opts, validations.MachineSetValidationOptions(innerSt, etcdBackupStoreFactory)...)
	opts = append(opts, validations.MachineSetNodeValidationOptions(innerSt)...)
	opts = append(opts, validations.KubernetesManifestsValidationOptions(innerSt)...)
	opts = append(opts, validations.KubernetesHealthCheckValidationOptions(innerSt)...)
	opts = append(opts, validations.RotateSecretsValidationOptions(innerSt)...)
	opts = append(opts, validations.EtcdManualBackupValidationOptions(innerSt)...)

	st := state.WrapCore(validated.NewState(innerSt, opts...))

	for version, k8sVersion := range map[string]string{"1.14.0": "1.34.0", "1.15.0": "1.35.0"} {
		talosVersion := omnires.NewTalosVersion(version)
		talosVersion.TypedSpec().Value.CompatibleKubernetesVersions = []string{k8sVersion}

		require.NoError(t, innerSt.Create(ctx, talosVersion))
	}

	return ctx, st
}

func newFeatureCluster(id, talosVersion string, kubernetes specs.FeatureState) *omnires.Cluster {
	k8sVersion := "1.35.0"
	if talosVersion == "1.14.0" {
		k8sVersion = "1.34.0"
	}

	cluster := omnires.NewCluster(id)
	cluster.TypedSpec().Value.TalosVersion = talosVersion
	cluster.TypedSpec().Value.KubernetesVersion = k8sVersion

	// a cluster without Kubernetes has no Kubernetes version
	if kubernetes == featureDisabled {
		cluster.TypedSpec().Value.KubernetesVersion = ""
	}

	cluster.TypedSpec().Value.Features = &specs.ClusterSpec_Features{Kubernetes: kubernetes}

	return cluster
}

func newFeatureMachineSet(cluster, id string, controlPlane bool, kubernetes specs.FeatureState) *omnires.MachineSet {
	machineSet := omnires.NewMachineSet(id)
	machineSet.Metadata().Labels().Set(omnires.LabelCluster, cluster)

	if controlPlane {
		machineSet.Metadata().Labels().Set(omnires.LabelControlPlaneRole, "")
	} else {
		machineSet.Metadata().Labels().Set(omnires.LabelWorkerRole, "")
	}

	machineSet.TypedSpec().Value.Kubernetes = kubernetes

	return machineSet
}

func newFeatureMachineSetNode(id string, machineSet *omnires.MachineSet, kubernetes specs.FeatureState) *omnires.MachineSetNode {
	node := omnires.NewMachineSetNode(id, machineSet)
	node.TypedSpec().Value.Kubernetes = kubernetes

	return node
}

func assertValidationError(t *testing.T, err error, contains string) {
	t.Helper()

	require.Error(t, err)
	assert.True(t, validated.IsValidationError(err), "expected validation error, got %v", err)
	assert.ErrorContains(t, err, contains)
}

func TestClusterKubernetesFeatureValidation(t *testing.T) {
	t.Parallel()

	ctx, st := newKubernetesFeatureState(t)

	t.Run("old talos", func(t *testing.T) {
		t.Parallel()

		assertValidationError(t, st.Create(ctx, newFeatureCluster("old-talos", "1.14.0", featureDisabled)), "requires Talos 1.15")
	})

	t.Run("kubernetes version", func(t *testing.T) {
		t.Parallel()

		cluster := newFeatureCluster("kubernetes-version", "1.15.0", featureDisabled)
		cluster.TypedSpec().Value.KubernetesVersion = "1.35.0"

		assertValidationError(t, st.Create(ctx, cluster), "can't have a Kubernetes version")
	})

	t.Run("workload proxy", func(t *testing.T) {
		t.Parallel()

		cluster := newFeatureCluster("workload-proxy", "1.15.0", featureDisabled)
		cluster.TypedSpec().Value.Features.EnableWorkloadProxy = true

		assertValidationError(t, st.Create(ctx, cluster), "workload proxy")
	})

	t.Run("backups", func(t *testing.T) {
		t.Parallel()

		cluster := newFeatureCluster("backups", "1.15.0", featureDisabled)
		cluster.TypedSpec().Value.BackupConfiguration = &specs.EtcdBackupConf{Enabled: true, Interval: durationpb.New(time.Hour)}

		assertValidationError(t, st.Create(ctx, cluster), "etcd backups")
	})

	t.Run("imported", func(t *testing.T) {
		t.Parallel()

		// a cluster without Kubernetes can be imported, and it has no initial Kubernetes version
		cluster := newFeatureCluster("imported", "1.15.0", featureDisabled)
		cluster.Metadata().Annotations().Set(omnires.ClusterImportIsInProgress, "")
		cluster.Metadata().Annotations().Set(omnires.ClusterImportInitialTalosVersion, "1.15.0")

		require.NoError(t, st.Create(ctx, cluster))
	})

	t.Run("imported from old talos", func(t *testing.T) {
		t.Parallel()

		// the config contract of an imported cluster comes from its initial Talos version, not the current one
		cluster := newFeatureCluster("imported-old", "1.15.0", featureDisabled)
		cluster.Metadata().Annotations().Set(omnires.ClusterImportIsInProgress, "")
		cluster.Metadata().Annotations().Set(omnires.ClusterImportInitialTalosVersion, "1.14.0")

		assertValidationError(t, st.Create(ctx, cluster), "requires Talos 1.15")
	})

	t.Run("immutable", func(t *testing.T) {
		t.Parallel()

		disabled := newFeatureCluster("immutable-disabled", "1.15.0", featureDisabled)
		require.NoError(t, st.Create(ctx, disabled))

		_, err := safe.StateUpdateWithConflicts(ctx, st, disabled.Metadata(), func(res *omnires.Cluster) error {
			res.TypedSpec().Value.Features.Kubernetes = featureEnabled

			return nil
		})
		assertValidationError(t, err, "can't be changed")

		_, err = safe.StateUpdateWithConflicts(ctx, st, disabled.Metadata(), func(res *omnires.Cluster) error {
			res.TypedSpec().Value.KubernetesVersion = "1.35.0"

			return nil
		})
		assertValidationError(t, err, "can't have a Kubernetes version")

		enabled := newFeatureCluster("immutable-enabled", "1.15.0", featureUnset)
		require.NoError(t, st.Create(ctx, enabled))

		_, err = safe.StateUpdateWithConflicts(ctx, st, enabled.Metadata(), func(res *omnires.Cluster) error {
			res.TypedSpec().Value.Features.Kubernetes = featureDisabled

			return nil
		})
		assertValidationError(t, err, "can't be changed")

		// unspecified and enabled mean the same
		_, err = safe.StateUpdateWithConflicts(ctx, st, enabled.Metadata(), func(res *omnires.Cluster) error {
			res.TypedSpec().Value.Features.Kubernetes = featureEnabled

			return nil
		})
		require.NoError(t, err)
	})

	t.Run("resources requiring kubernetes", func(t *testing.T) {
		t.Parallel()

		require.NoError(t, st.Create(ctx, newFeatureCluster("no-k8s-resources", "1.15.0", featureDisabled)))

		manifest := omnires.NewKubernetesManifestGroup("no-k8s-resources-manifest")
		manifest.TypedSpec().Value.Mode = specs.KubernetesManifestGroupSpec_ONE_TIME
		manifest.Metadata().Labels().Set(omnires.LabelCluster, "no-k8s-resources")

		assertValidationError(t, st.Create(ctx, manifest), "Kubernetes is disabled")

		assertValidationError(t, st.Create(ctx, omnires.NewRotateKubernetesCA("no-k8s-resources")), "Kubernetes is disabled")

		manualBackup := omnires.NewEtcdManualBackup("no-k8s-resources")
		manualBackup.TypedSpec().Value.BackupAt = timestamppb.Now()

		assertValidationError(t, st.Create(ctx, manualBackup), "Kubernetes is disabled")

		healthCheck := omnires.NewKubernetesHealthCheck("no-k8s-resources-check")
		healthCheck.TypedSpec().Value.Job = "apiVersion: batch/v1\nkind: Job\nspec:\n  template:\n    spec:\n      containers:\n        - name: check\n          image: alpine\n"
		healthCheck.Metadata().Labels().Set(omnires.LabelCluster, "no-k8s-resources")

		assertValidationError(t, st.Create(ctx, healthCheck), "Kubernetes is disabled")
	})

	t.Run("resources requiring kubernetes created before the cluster", func(t *testing.T) {
		t.Parallel()

		manifest := newFeatureManifest("early-manifest", "early-manifest")
		require.NoError(t, st.Create(ctx, manifest))

		assertValidationError(t, st.Create(ctx, newFeatureCluster("early-manifest", "1.15.0", featureDisabled)), "it has Kubernetes manifests")

		healthCheck := newFeatureHealthCheck("early-check", "early-check")
		require.NoError(t, st.Create(ctx, healthCheck))

		assertValidationError(t, st.Create(ctx, newFeatureCluster("early-check", "1.15.0", featureDisabled)), "it has Kubernetes healthchecks")

		require.NoError(t, st.Create(ctx, omnires.NewRotateKubernetesCA("early-rotation")))

		assertValidationError(t, st.Create(ctx, newFeatureCluster("early-rotation", "1.15.0", featureDisabled)), "it has a Kubernetes CA rotation")

		earlyBackup := omnires.NewEtcdManualBackup("early-backup")
		earlyBackup.TypedSpec().Value.BackupAt = timestamppb.Now()
		require.NoError(t, st.Create(ctx, earlyBackup))

		assertValidationError(t, st.Create(ctx, newFeatureCluster("early-backup", "1.15.0", featureDisabled)), "it has a manual etcd backup")

		// a cluster with Kubernetes can have them
		require.NoError(t, st.Create(ctx, newFeatureCluster("early-manifest", "1.15.0", featureUnset)))
	})

	t.Run("resources requiring kubernetes moved to a cluster without kubernetes", func(t *testing.T) {
		t.Parallel()

		require.NoError(t, st.Create(ctx, newFeatureCluster("move-from", "1.15.0", featureUnset)))
		require.NoError(t, st.Create(ctx, newFeatureCluster("move-to", "1.15.0", featureDisabled)))

		manifest := newFeatureManifest("move-manifest", "move-from")
		require.NoError(t, st.Create(ctx, manifest))

		_, err := safe.StateUpdateWithConflicts(ctx, st, manifest.Metadata(), func(res *omnires.KubernetesManifestGroup) error {
			res.Metadata().Labels().Set(omnires.LabelCluster, "move-to")

			return nil
		})
		assertValidationError(t, err, "Kubernetes is disabled")

		healthCheck := newFeatureHealthCheck("move-check", "move-from")
		require.NoError(t, st.Create(ctx, healthCheck))

		_, err = safe.StateUpdateWithConflicts(ctx, st, healthCheck.Metadata(), func(res *omnires.KubernetesHealthCheck) error {
			res.Metadata().Labels().Set(omnires.LabelCluster, "move-to")

			return nil
		})
		assertValidationError(t, err, "Kubernetes is disabled")

		// updates which keep the cluster still work
		_, err = safe.StateUpdateWithConflicts(ctx, st, manifest.Metadata(), func(res *omnires.KubernetesManifestGroup) error {
			res.TypedSpec().Value.Mode = specs.KubernetesManifestGroupSpec_FULL

			return nil
		})
		require.NoError(t, err)
	})
}

func newFeatureManifest(id, cluster string) *omnires.KubernetesManifestGroup {
	manifest := omnires.NewKubernetesManifestGroup(id)
	manifest.TypedSpec().Value.Mode = specs.KubernetesManifestGroupSpec_ONE_TIME
	manifest.Metadata().Labels().Set(omnires.LabelCluster, cluster)

	return manifest
}

func newFeatureHealthCheck(id, cluster string) *omnires.KubernetesHealthCheck {
	healthCheck := omnires.NewKubernetesHealthCheck(id)
	healthCheck.TypedSpec().Value.Job = "apiVersion: batch/v1\nkind: Job\nspec:\n  template:\n    spec:\n      containers:\n        - name: check\n          image: alpine\n"
	healthCheck.Metadata().Labels().Set(omnires.LabelCluster, cluster)

	return healthCheck
}

func TestMachineSetKubernetesFeatureValidation(t *testing.T) {
	t.Parallel()

	ctx, st := newKubernetesFeatureState(t)

	require.NoError(t, st.Create(ctx, newFeatureCluster("k8s", "1.15.0", featureUnset)))
	require.NoError(t, st.Create(ctx, newFeatureCluster("old", "1.14.0", featureUnset)))
	require.NoError(t, st.Create(ctx, newFeatureCluster("no-k8s", "1.15.0", featureDisabled)))

	assertValidationError(t,
		st.Create(ctx, newFeatureMachineSet("k8s", omnires.ControlPlanesResourceID("k8s"), true, featureDisabled)),
		"control plane machine set",
	)

	assertValidationError(t,
		st.Create(ctx, newFeatureMachineSet("no-k8s", "no-k8s-workers", false, featureEnabled)),
		"disabled for the cluster",
	)

	assertValidationError(t,
		st.Create(ctx, newFeatureMachineSet("old", "old-workers", false, featureDisabled)),
		"requires Talos 1.15",
	)

	// the cluster was upgraded to Talos 1.15, but its config contract is still pinned to 1.14
	require.NoError(t, st.Create(ctx, newFeatureCluster("upgraded", "1.15.0", featureUnset)))

	configVersion := omnires.NewClusterConfigVersion("upgraded")
	configVersion.TypedSpec().Value.Version = "v1.14.0"
	require.NoError(t, st.Create(ctx, configVersion))

	assertValidationError(t,
		st.Create(ctx, newFeatureMachineSet("upgraded", "upgraded-workers", false, featureDisabled)),
		"requires Talos 1.15",
	)

	// a cluster without Kubernetes has no etcd to restore from a backup
	require.NoError(t, st.Create(ctx, newFeatureCluster("no-k8s-restore", "1.15.0", featureDisabled)))

	restored := newFeatureMachineSet("no-k8s-restore", omnires.ControlPlanesResourceID("no-k8s-restore"), true, featureUnset)
	restored.TypedSpec().Value.BootstrapSpec = &specs.MachineSetSpec_BootstrapSpec{ClusterUuid: "uuid", Snapshot: "snapshot"}

	assertValidationError(t, st.Create(ctx, restored), "bootstrap spec")

	// a cluster without Kubernetes can have its control planes or not, and they don't run Kubernetes
	require.NoError(t, st.Create(ctx, newFeatureMachineSet("no-k8s", omnires.ControlPlanesResourceID("no-k8s"), true, featureUnset)))

	workers := newFeatureMachineSet("k8s", "k8s-workers", false, featureDisabled)
	require.NoError(t, st.Create(ctx, workers))

	_, err := safe.StateUpdateWithConflicts(ctx, st, workers.Metadata(), func(res *omnires.MachineSet) error {
		res.TypedSpec().Value.Kubernetes = featureUnset

		return nil
	})
	assertValidationError(t, err, "can't be changed")
}

func TestMachineSetNodeKubernetesFeatureValidation(t *testing.T) {
	t.Parallel()

	ctx, st := newKubernetesFeatureState(t)

	require.NoError(t, st.Create(ctx, newFeatureCluster("k8s", "1.15.0", featureUnset)))

	controlPlanes := newFeatureMachineSet("k8s", omnires.ControlPlanesResourceID("k8s"), true, featureUnset)
	disabledWorkers := newFeatureMachineSet("k8s", "k8s-disabled", false, featureDisabled)
	workers := newFeatureMachineSet("k8s", "k8s-workers", false, featureUnset)

	for _, machineSet := range []*omnires.MachineSet{controlPlanes, disabledWorkers, workers} {
		require.NoError(t, st.Create(ctx, machineSet))
	}

	assertValidationError(t,
		st.Create(ctx, newFeatureMachineSetNode("cp-1", controlPlanes, featureDisabled)),
		"control plane machine",
	)

	assertValidationError(t,
		st.Create(ctx, newFeatureMachineSetNode("disabled-1", disabledWorkers, featureEnabled)),
		"disabled for its machine set",
	)

	require.NoError(t, st.Create(ctx, newFeatureCluster("upgraded", "1.15.0", featureUnset)))

	configVersion := omnires.NewClusterConfigVersion("upgraded")
	configVersion.TypedSpec().Value.Version = "v1.14.0"
	require.NoError(t, st.Create(ctx, configVersion))

	upgradedWorkers := newFeatureMachineSet("upgraded", "upgraded-workers", false, featureUnset)
	require.NoError(t, st.Create(ctx, upgradedWorkers))

	assertValidationError(t,
		st.Create(ctx, newFeatureMachineSetNode("upgraded-1", upgradedWorkers, featureDisabled)),
		"requires Talos 1.15",
	)

	require.NoError(t, st.Create(ctx, newFeatureMachineSetNode("disabled-2", disabledWorkers, featureUnset)))
	require.NoError(t, st.Create(ctx, newFeatureMachineSetNode("disabled-3", disabledWorkers, featureDisabled)))

	worker := newFeatureMachineSetNode("worker-1", workers, featureDisabled)
	require.NoError(t, st.Create(ctx, worker))

	_, err := safe.StateUpdateWithConflicts(ctx, st, worker.Metadata(), func(res *omnires.MachineSetNode) error {
		res.TypedSpec().Value.Kubernetes = featureEnabled

		return nil
	})
	assertValidationError(t, err, "can't be changed")
}
