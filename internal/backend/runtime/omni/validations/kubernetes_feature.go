// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package validations

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	machineryconfig "github.com/siderolabs/talos/pkg/machinery/config"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// validateClusterKubernetesFeature validates the Kubernetes feature of the cluster.
//
// The feature is immutable: Kubernetes can't be enabled or disabled after the cluster is created.
func validateClusterKubernetesFeature(oldRes, newRes *omni.Cluster) error {
	enabled := omni.IsKubernetesEnabled(newRes)

	if oldRes != nil && omni.IsKubernetesEnabled(oldRes) != enabled {
		return errors.New("the Kubernetes feature of the cluster can't be changed after the cluster is created")
	}

	if enabled {
		return nil
	}

	if newRes.TypedSpec().Value.GetKubernetesVersion() != "" {
		return errors.New("a cluster without Kubernetes can't have a Kubernetes version")
	}

	if newRes.TypedSpec().Value.GetFeatures().GetEnableWorkloadProxy() {
		return errors.New("workload proxy can't be enabled for a cluster without Kubernetes")
	}

	if newRes.TypedSpec().Value.GetBackupConfiguration().GetEnabled() {
		return errors.New("etcd backups can't be enabled for a cluster without Kubernetes")
	}

	if oldRes == nil {
		// the feature is immutable, so the Talos version is checked only on create:
		// the initial Talos version defines the config contract the machines are generated with
		if err := validateKubernetesDisabledTalosVersion(initialClusterTalosVersion(newRes)); err != nil {
			return err
		}

		return nil
	}

	return nil
}

// initialClusterTalosVersion returns the Talos version the cluster config contract is created from.
//
// It mirrors the ClusterConfigVersion controller: an imported cluster uses its initial Talos version.
func initialClusterTalosVersion(cluster *omni.Cluster) string {
	if initialVersion, ok := omni.GetImportInitialVersion(cluster, omni.ClusterImportInitialTalosVersion); ok {
		return initialVersion
	}

	return cluster.TypedSpec().Value.GetTalosVersion()
}

// clusterConfigTalosVersion returns the Talos version the config contract of the existing cluster is pinned to.
//
// The contract is pinned when the cluster is created, so a cluster upgraded to Talos 1.15 can still generate configs with an older contract.
func clusterConfigTalosVersion(ctx context.Context, st state.State, cluster *omni.Cluster) (string, error) {
	configVersion, err := safe.StateGetByID[*omni.ClusterConfigVersion](ctx, st, cluster.Metadata().ID())
	if err != nil && !state.IsNotFoundError(err) {
		return "", err
	}

	if configVersion != nil && configVersion.TypedSpec().Value.GetVersion() != "" {
		return strings.TrimPrefix(configVersion.TypedSpec().Value.GetVersion(), "v"), nil
	}

	return initialClusterTalosVersion(cluster), nil
}

func validateKubernetesDisabledTalosVersion(talosVersion string) error {
	versionContract, err := machineryconfig.ParseContractFromVersion(talosVersion)
	if err != nil {
		return fmt.Errorf("failed to parse Talos version %q: %w", talosVersion, err)
	}

	if !versionContract.Greater(machineryconfig.TalosVersion1_14) {
		return fmt.Errorf("disabling Kubernetes requires Talos 1.15 or newer, got %q", talosVersion)
	}

	return nil
}

func validateKubernetesDisabledClusterConfigVersion(ctx context.Context, st state.State, cluster *omni.Cluster) error {
	talosVersion, err := clusterConfigTalosVersion(ctx, st, cluster)
	if err != nil {
		return err
	}

	return validateKubernetesDisabledTalosVersion(talosVersion)
}

// validateMachineSetKubernetesFeature validates the Kubernetes feature of the machine set.
func validateMachineSetKubernetesFeature(ctx context.Context, st state.State, cluster *omni.Cluster, oldRes, newRes *omni.MachineSet) error {
	featureState := newRes.TypedSpec().Value.GetKubernetes()

	// a cluster without Kubernetes has no etcd, so there is nothing to restore from a backup
	if !omni.IsKubernetesEnabled(cluster) && newRes.TypedSpec().Value.GetBootstrapSpec() != nil {
		return errors.New("a machine set of a cluster without Kubernetes can't have a bootstrap spec")
	}

	if oldRes != nil && omni.ResolveKubernetes(cluster, oldRes, nil) != omni.ResolveKubernetes(cluster, newRes, nil) {
		return errors.New("the Kubernetes feature of the machine set can't be changed after the machine set is created")
	}

	if featureState == specs.FeatureState_FEATURE_STATE_UNSPECIFIED {
		return nil
	}

	clusterEnabled := omni.IsKubernetesEnabled(cluster)

	switch featureState { //nolint:exhaustive // the unspecified state is handled above
	case specs.FeatureState_FEATURE_STATE_ENABLED:
		if !clusterEnabled {
			return errors.New("Kubernetes can't be enabled for a machine set when it is disabled for the cluster") //nolint:staticcheck
		}
	case specs.FeatureState_FEATURE_STATE_DISABLED:
		if isControlPlaneMachineSet(newRes) && clusterEnabled {
			return errors.New("Kubernetes can't be disabled for the control plane machine set of a cluster with Kubernetes") //nolint:staticcheck
		}
	}

	if oldRes == nil && featureState == specs.FeatureState_FEATURE_STATE_DISABLED && cluster != nil {
		return validateKubernetesDisabledClusterConfigVersion(ctx, st, cluster)
	}

	return nil
}

// validateMachineSetNodeKubernetesFeature validates the Kubernetes feature of the machine set node.
func validateMachineSetNodeKubernetesFeature(ctx context.Context, st state.State, cluster *omni.Cluster, machineSet *omni.MachineSet, oldRes, newRes *omni.MachineSetNode) error {
	featureState := newRes.TypedSpec().Value.GetKubernetes()

	if oldRes != nil && oldRes.TypedSpec().Value.GetKubernetes() != featureState &&
		omni.ResolveKubernetes(cluster, machineSet, oldRes) != omni.ResolveKubernetes(cluster, machineSet, newRes) {
		return errors.New("the Kubernetes feature of the machine can't be changed after the machine is added to the cluster")
	}

	if featureState == specs.FeatureState_FEATURE_STATE_UNSPECIFIED {
		return nil
	}

	parentEnabled := omni.ResolveKubernetes(cluster, machineSet, nil)

	switch featureState { //nolint:exhaustive // the unspecified state is handled above
	case specs.FeatureState_FEATURE_STATE_ENABLED:
		if !parentEnabled {
			return errors.New("Kubernetes can't be enabled for a machine when it is disabled for its machine set or cluster") //nolint:staticcheck
		}
	case specs.FeatureState_FEATURE_STATE_DISABLED:
		if isControlPlaneMachineSet(machineSet) && omni.IsKubernetesEnabled(cluster) {
			return errors.New("Kubernetes can't be disabled for a control plane machine of a cluster with Kubernetes") //nolint:staticcheck
		}
	}

	if oldRes == nil && featureState == specs.FeatureState_FEATURE_STATE_DISABLED && cluster != nil {
		return validateKubernetesDisabledClusterConfigVersion(ctx, st, cluster)
	}

	return nil
}

// validateClusterRunsKubernetes returns an error if the cluster the resource belongs to has Kubernetes disabled.
//
// A missing cluster is not an error: the resources can be created before the cluster.
func validateClusterRunsKubernetes(ctx context.Context, st state.State, clusterName resource.ID, what string) error {
	if clusterName == "" {
		return nil
	}

	cluster, err := safe.StateGetByID[*omni.Cluster](ctx, st, clusterName)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil
		}

		return err
	}

	if !omni.IsKubernetesEnabled(cluster) {
		return fmt.Errorf("%s can't be used with the cluster %q: Kubernetes is disabled for the cluster", what, clusterName)
	}

	return nil
}

// validateClusterLabelRunsKubernetes checks that a resource moved to another cluster by an update doesn't move to a cluster with Kubernetes disabled.
func validateClusterLabelRunsKubernetes(ctx context.Context, st state.State, oldRes, newRes resource.Resource, what string) error {
	oldCluster, _ := oldRes.Metadata().Labels().Get(omni.LabelCluster)
	newCluster, _ := newRes.Metadata().Labels().Get(omni.LabelCluster)

	if oldCluster == newCluster {
		return nil
	}

	return validateClusterRunsKubernetes(ctx, st, newCluster, what)
}

// validateNoResourcesRequireKubernetes checks that no resource which requires Kubernetes was created for the cluster before it.
//
// Such resources can be created before their cluster, so creating a cluster with Kubernetes disabled has to check them:
// the feature can't be changed after that.
func validateNoResourcesRequireKubernetes(ctx context.Context, st state.State, clusterName resource.ID) error {
	clusterQuery := state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, clusterName))

	manifests, err := safe.StateListAll[*omni.KubernetesManifestGroup](ctx, st, clusterQuery)
	if err != nil {
		return err
	}

	if manifests.Len() > 0 {
		return fmt.Errorf("Kubernetes can't be disabled for the cluster %q: it has Kubernetes manifests", clusterName) //nolint:staticcheck
	}

	healthChecks, err := safe.StateListAll[*omni.KubernetesHealthCheck](ctx, st, clusterQuery)
	if err != nil {
		return err
	}

	if healthChecks.Len() > 0 {
		return fmt.Errorf("Kubernetes can't be disabled for the cluster %q: it has Kubernetes healthchecks", clusterName) //nolint:staticcheck
	}

	if _, err = safe.StateGetByID[*omni.RotateKubernetesCA](ctx, st, clusterName); err == nil {
		return fmt.Errorf("Kubernetes can't be disabled for the cluster %q: it has a Kubernetes CA rotation", clusterName) //nolint:staticcheck
	} else if !state.IsNotFoundError(err) {
		return err
	}

	if _, err = safe.StateGetByID[*omni.EtcdManualBackup](ctx, st, clusterName); err == nil {
		return fmt.Errorf("Kubernetes can't be disabled for the cluster %q: it has a manual etcd backup", clusterName) //nolint:staticcheck
	} else if !state.IsNotFoundError(err) {
		return err
	}

	return nil
}

func isControlPlaneMachineSet(machineSet *omni.MachineSet) bool {
	if machineSet == nil {
		return false
	}

	_, isControlPlane := machineSet.Metadata().Labels().Get(omni.LabelControlPlaneRole)

	return isControlPlane
}
