// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package omni_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

func TestResolveKubernetes(t *testing.T) {
	t.Parallel()

	const (
		unset    = specs.FeatureState_FEATURE_STATE_UNSPECIFIED
		enabled  = specs.FeatureState_FEATURE_STATE_ENABLED
		disabled = specs.FeatureState_FEATURE_STATE_DISABLED
	)

	for _, tt := range []struct {
		name       string
		cluster    specs.FeatureState
		machineSet specs.FeatureState
		node       specs.FeatureState
		expected   bool
	}{
		{name: "all unset", expected: true},
		{name: "cluster enabled", cluster: enabled, expected: true},
		{name: "cluster disabled", cluster: disabled, expected: false},
		{name: "machine set disabled", machineSet: disabled, expected: false},
		{name: "machine set overrides cluster", cluster: disabled, machineSet: enabled, expected: true},
		{name: "node inherits machine set", machineSet: disabled, node: unset, expected: false},
		{name: "node overrides machine set", machineSet: disabled, node: enabled, expected: true},
		{name: "node disabled", cluster: enabled, node: disabled, expected: false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cluster := omni.NewCluster("test")
			cluster.TypedSpec().Value.Features = &specs.ClusterSpec_Features{Kubernetes: tt.cluster}

			machineSet := omni.NewMachineSet("test-workers")
			machineSet.TypedSpec().Value.Kubernetes = tt.machineSet

			node := omni.NewMachineSetNode("machine", machineSet)
			node.TypedSpec().Value.Kubernetes = tt.node

			assert.Equal(t, tt.expected, omni.ResolveKubernetes(cluster, machineSet, node))
		})
	}
}

func TestIsKubernetesEnabled(t *testing.T) {
	t.Parallel()

	assert.True(t, omni.IsKubernetesEnabled(nil))

	cluster := omni.NewCluster("test")
	assert.True(t, omni.IsKubernetesEnabled(cluster))

	cluster.TypedSpec().Value.Features = &specs.ClusterSpec_Features{Kubernetes: specs.FeatureState_FEATURE_STATE_DISABLED}
	assert.False(t, omni.IsKubernetesEnabled(cluster))
}

func TestRunsEtcd(t *testing.T) {
	t.Parallel()

	cm := omni.NewClusterMachine("machine")

	assert.False(t, omni.RunsKubernetes(cm))
	assert.False(t, omni.RunsEtcd(cm))

	cm.Metadata().Labels().Set(omni.LabelKubernetes, "")

	assert.True(t, omni.RunsKubernetes(cm))
	assert.False(t, omni.RunsEtcd(cm), "workers never run etcd")

	cm.Metadata().Labels().Set(omni.LabelControlPlaneRole, "")

	assert.True(t, omni.RunsEtcd(cm))

	cm.Metadata().Labels().Delete(omni.LabelKubernetes)

	assert.False(t, omni.RunsEtcd(cm), "etcd is disabled together with Kubernetes")
}
