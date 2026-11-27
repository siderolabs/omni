// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package omni

import (
	"github.com/cosi-project/runtime/pkg/resource"

	"github.com/siderolabs/omni/client/api/omni/specs"
)

// ClusterKubernetesState returns the Kubernetes feature state set on the cluster.
func ClusterKubernetesState(cluster *Cluster) specs.FeatureState {
	if cluster == nil {
		return specs.FeatureState_FEATURE_STATE_UNSPECIFIED
	}

	return cluster.TypedSpec().Value.GetFeatures().GetKubernetes()
}

// IsKubernetesEnabled returns true if the cluster runs Kubernetes.
//
// Kubernetes is enabled unless it is explicitly disabled.
func IsKubernetesEnabled(cluster *Cluster) bool {
	return ClusterKubernetesState(cluster) != specs.FeatureState_FEATURE_STATE_DISABLED
}

// ResolveKubernetes returns true if a machine in the given machine set (and with the given machine set node) runs Kubernetes.
//
// The most specific level which has the feature state set wins. The machine set and the machine set node can be nil.
func ResolveKubernetes(cluster *Cluster, machineSet *MachineSet, machineSetNode *MachineSetNode) bool {
	state := ClusterKubernetesState(cluster)

	if machineSet != nil {
		state = resolveFeatureState(state, machineSet.TypedSpec().Value.GetKubernetes())
	}

	if machineSetNode != nil {
		state = resolveFeatureState(state, machineSetNode.TypedSpec().Value.GetKubernetes())
	}

	return state != specs.FeatureState_FEATURE_STATE_DISABLED
}

func resolveFeatureState(parent, child specs.FeatureState) specs.FeatureState {
	if child == specs.FeatureState_FEATURE_STATE_UNSPECIFIED {
		return parent
	}

	return child
}

// RunsKubernetes returns true if the resource (ClusterMachine or ClusterMachineStatus) belongs to a machine which runs Kubernetes.
func RunsKubernetes(res resource.Resource) bool {
	_, ok := res.Metadata().Labels().Get(LabelKubernetes)

	return ok
}

// RunsEtcd returns true if the resource (ClusterMachine or ClusterMachineStatus) belongs to a machine which runs etcd.
//
// Talos runs etcd only on the control plane machines, and etcd is disabled together with Kubernetes.
func RunsEtcd(res resource.Resource) bool {
	_, isControlPlane := res.Metadata().Labels().Get(LabelControlPlaneRole)

	return isControlPlane && RunsKubernetes(res)
}
