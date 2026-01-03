// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package kubernetes_test

import (
	"fmt"
	"slices"
	"testing"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/kubernetes"
)

//nolint:maintidx
func TestUpgradePath(t *testing.T) {
	t.Parallel()

	kubernetesStatus := func(clusterID resource.ID, spec *specs.KubernetesStatusSpec) *omni.KubernetesStatus {
		r := omni.NewKubernetesStatus(clusterID)
		r.TypedSpec().Value = spec

		return r
	}

	requiredImages := func(desiredVersion string, controlPlane bool) []string {
		images := []string{
			fmt.Sprintf("%s:v%s", constants.KubeletImage, desiredVersion),
		}

		if controlPlane {
			images = append(images,
				fmt.Sprintf("%s:v%s", constants.KubeProxyImage, desiredVersion),
				fmt.Sprintf("%s:v%s", constants.KubernetesSchedulerImage, desiredVersion),
				fmt.Sprintf("%s:v%s", constants.KubernetesAPIServerImage, desiredVersion),
				fmt.Sprintf("%s:v%s", constants.KubernetesControllerManagerImage, desiredVersion),
			)
		}

		slices.Sort(images)

		return images
	}

	for _, test := range []struct { //nolint:govet
		name             string
		machineMap       *kubernetes.MachineMap
		kubernetesStatus *omni.KubernetesStatus
		desiredVersion   string

		expected *kubernetes.UpgradePath
	}{
		{
			name: "no upgrade",
			machineMap: &kubernetes.MachineMap{
				ControlPlanes: map[string]string{
					"cp1": "node-cp-1",
					"cp2": "node-cp-2",
					"cp3": "node-cp-3",
				},
				Workers: map[string]string{
					"worker1": "node-worker-1",
				},
			},
			kubernetesStatus: kubernetesStatus("cluster1", &specs.KubernetesStatusSpec{
				Nodes: []*specs.KubernetesStatusSpec_NodeStatus{
					{
						Nodename:       "cp1",
						KubeletVersion: "1.20.1",
						Ready:          true,
					},
					{
						Nodename:       "cp2",
						KubeletVersion: "1.20.1",
						Ready:          true,
					},
					{
						Nodename:       "cp3",
						KubeletVersion: "1.20.1",
						Ready:          true,
					},
					{
						Nodename:       "worker1",
						KubeletVersion: "1.20.1",
						Ready:          true,
					},
				},
				StaticPods: []*specs.KubernetesStatusSpec_NodeStaticPods{
					{
						Nodename: "cp1",
						StaticPods: []*specs.KubernetesStatusSpec_StaticPodStatus{
							{
								App:     "kube-apiserver",
								Version: "1.20.1",
								Ready:   true,
							},
							{
								App:     "kube-controller-manager",
								Version: "1.20.1",
								Ready:   false,
							},
							{
								App:     "kube-scheduler",
								Version: "1.20.1",
								Ready:   true,
							},
						},
					},
					{
						Nodename: "cp2",
						StaticPods: []*specs.KubernetesStatusSpec_StaticPodStatus{
							{
								App:     "kube-apiserver",
								Version: "1.20.1",
								Ready:   true,
							},
							{
								App:     "kube-controller-manager",
								Version: "1.20.1",
								Ready:   true,
							},
							{
								App:     "kube-scheduler",
								Version: "1.20.1",
								Ready:   true,
							},
						},
					},
					{
						Nodename: "cp3",
						StaticPods: []*specs.KubernetesStatusSpec_StaticPodStatus{
							{
								App:     "kube-apiserver",
								Version: "1.20.1",
								Ready:   true,
							},
							{
								App:     "kube-controller-manager",
								Version: "1.20.1",
								Ready:   true,
							},
							// kube-scheduler is missing
						},
					},
				},
			}),
			desiredVersion: "1.20.1",
			expected: &kubernetes.UpgradePath{
				ClusterID:          "cluster1",
				AllComponentsReady: false,
				NotReadyStatus:     "static pods [\"cp1/kube-controller-manager\" \"cp3/kube-scheduler\"] are not ready",
				AllNodes:           []string{"cp1", "cp2", "cp3", "worker1"},
				AllNodesToRequiredImages: map[string][]string{
					"cp1":     requiredImages("1.20.1", true),
					"cp2":     requiredImages("1.20.1", true),
					"cp3":     requiredImages("1.20.1", true),
					"worker1": requiredImages("1.20.1", false),
				},
				Steps: []kubernetes.UpgradeStep{
					{
						MachineID:   "node-cp-1",
						Description: "cp1: updating kube-controller-manager to 1.20.1",
						Node:        "cp1",
						Component:   kubernetes.ControllerManager,
					},
					{
						MachineID:   "node-cp-3",
						Description: "cp3: updating kube-scheduler to 1.20.1",
						Node:        "cp3",
						Component:   kubernetes.Scheduler,
					},
				},
			},
		},
		{
			name: "all healthy",
			machineMap: &kubernetes.MachineMap{
				ControlPlanes: map[string]string{
					"cp1": "node-cp-1",
				},
				Workers: map[string]string{
					"worker1": "node-worker-1",
				},
			},
			kubernetesStatus: kubernetesStatus("cluster2", &specs.KubernetesStatusSpec{
				Nodes: []*specs.KubernetesStatusSpec_NodeStatus{
					{
						Nodename:       "cp1",
						KubeletVersion: "1.20.1",
						Ready:          true,
					},
					{
						Nodename:       "worker1",
						KubeletVersion: "1.20.1",
						Ready:          true,
					},
				},
				StaticPods: []*specs.KubernetesStatusSpec_NodeStaticPods{
					{
						Nodename: "cp1",
						StaticPods: []*specs.KubernetesStatusSpec_StaticPodStatus{
							{
								App:     "kube-apiserver",
								Version: "1.20.1",
								Ready:   true,
							},
							{
								App:     "kube-controller-manager",
								Version: "1.20.1",
								Ready:   true,
							},
							{
								App:     "kube-scheduler",
								Version: "1.20.1",
								Ready:   true,
							},
						},
					},
				},
			}),
			desiredVersion: "1.20.1",
			expected: &kubernetes.UpgradePath{
				ClusterID:          "cluster2",
				AllComponentsReady: true,
				AllNodes:           []string{"cp1", "worker1"},
				AllNodesToRequiredImages: map[string][]string{
					"cp1":     requiredImages("1.20.1", true),
					"worker1": requiredImages("1.20.1", false),
				},
			},
		},
		{
			name: "node unhealthy",
			machineMap: &kubernetes.MachineMap{
				ControlPlanes: map[string]string{
					"cp1": "node-cp-1",
				},
				Workers: map[string]string{
					"worker1": "node-worker-1",
				},
			},
			kubernetesStatus: kubernetesStatus("cluster3", &specs.KubernetesStatusSpec{
				Nodes: []*specs.KubernetesStatusSpec_NodeStatus{
					{
						Nodename:       "cp1",
						KubeletVersion: "1.20.1",
						Ready:          false,
					},
					// worker1 is missing
				},
				StaticPods: []*specs.KubernetesStatusSpec_NodeStaticPods{
					{
						Nodename: "cp1",
						StaticPods: []*specs.KubernetesStatusSpec_StaticPodStatus{
							{
								App:     "kube-apiserver",
								Version: "1.20.1",
								Ready:   true,
							},
							{
								App:     "kube-controller-manager",
								Version: "1.20.1",
								Ready:   true,
							},
							{
								App:     "kube-scheduler",
								Version: "1.20.1",
								Ready:   true,
							},
						},
					},
				},
			}),
			desiredVersion: "1.20.1",
			expected: &kubernetes.UpgradePath{
				ClusterID:          "cluster3",
				AllComponentsReady: false,
				NotReadyStatus:     "nodes [\"cp1\" \"worker1\"] are not ready",
				AllNodes:           []string{"cp1", "worker1"},
				AllNodesToRequiredImages: map[string][]string{
					"cp1":     requiredImages("1.20.1", true),
					"worker1": requiredImages("1.20.1", false),
				},
				Steps: []kubernetes.UpgradeStep{
					{
						MachineID:   "node-cp-1",
						Description: "cp1: updating kubelet to 1.20.1",
						Node:        "cp1",
						Component:   kubernetes.Kubelet,
					},
					{
						MachineID:   "node-worker-1",
						Description: "worker1: updating kubelet to 1.20.1",
						Node:        "worker1",
						Component:   kubernetes.Kubelet,
					},
				},
			},
		},
		{
			name: "upgrade plan",
			machineMap: &kubernetes.MachineMap{
				ControlPlanes: map[string]string{
					"cp1": "node-cp-1",
					"cp2": "node-cp-2",
					"cp3": "node-cp-3",
				},
				Workers: map[string]string{
					"worker1": "node-worker-1",
				},
			},
			kubernetesStatus: kubernetesStatus("cluster4", &specs.KubernetesStatusSpec{
				Nodes: []*specs.KubernetesStatusSpec_NodeStatus{
					{
						Nodename:       "cp1",
						KubeletVersion: "1.20.2",
						Ready:          true,
					},
					{
						Nodename:       "cp2",
						KubeletVersion: "1.20.2",
						Ready:          true,
					},
					{
						Nodename:       "cp3",
						KubeletVersion: "1.20.1",
						Ready:          true,
					},
					{
						Nodename:       "worker1",
						KubeletVersion: "1.20.1",
						Ready:          true,
					},
				},
				StaticPods: []*specs.KubernetesStatusSpec_NodeStaticPods{
					{
						Nodename: "cp1",
						StaticPods: []*specs.KubernetesStatusSpec_StaticPodStatus{
							{
								App:     "kube-apiserver",
								Version: "1.20.2",
								Ready:   true,
							},
							{
								App:     "kube-controller-manager",
								Version: "1.20.2",
								Ready:   true,
							},
							{
								App:     "kube-scheduler",
								Version: "1.20.1",
								Ready:   true,
							},
						},
					},
					{
						Nodename: "cp2",
						StaticPods: []*specs.KubernetesStatusSpec_StaticPodStatus{
							{
								App:     "kube-apiserver",
								Version: "1.20.2",
								Ready:   true,
							},
							{
								App:     "kube-controller-manager",
								Version: "1.20.2",
								Ready:   true,
							},
							{
								App:     "kube-scheduler",
								Version: "1.20.1",
								Ready:   true,
							},
						},
					},
					{
						Nodename: "cp3",
						StaticPods: []*specs.KubernetesStatusSpec_StaticPodStatus{
							{
								App:     "kube-apiserver",
								Version: "1.20.2",
								Ready:   true,
							},
							{
								App:     "kube-controller-manager",
								Version: "1.20.1",
								Ready:   true,
							},
							{
								App:     "kube-scheduler",
								Version: "1.20.1",
								Ready:   true,
							},
						},
					},
				},
			}),
			desiredVersion: "1.20.2",
			expected: &kubernetes.UpgradePath{
				ClusterID:          "cluster4",
				AllComponentsReady: true,
				AllNodes:           []string{"cp1", "cp2", "cp3", "worker1"},
				AllNodesToRequiredImages: map[string][]string{
					"cp1":     requiredImages("1.20.2", true),
					"cp2":     requiredImages("1.20.2", true),
					"cp3":     requiredImages("1.20.2", true),
					"worker1": requiredImages("1.20.2", false),
				},
				Steps: []kubernetes.UpgradeStep{
					{
						MachineID:   "node-cp-3",
						Description: "cp3: updating kube-controller-manager to 1.20.2",
						Node:        "cp3",
						Component:   kubernetes.ControllerManager,
					},
					{
						MachineID:   "node-cp-1",
						Description: "cp1: updating kube-scheduler to 1.20.2",
						Node:        "cp1",
						Component:   kubernetes.Scheduler,
					},
					{
						MachineID:   "node-cp-2",
						Description: "cp2: updating kube-scheduler to 1.20.2",
						Node:        "cp2",
						Component:   kubernetes.Scheduler,
					},
					{
						MachineID:   "node-cp-3",
						Description: "cp3: updating kube-scheduler to 1.20.2",
						Node:        "cp3",
						Component:   kubernetes.Scheduler,
					},
					{
						MachineID:   "node-cp-3",
						Description: "cp3: updating kubelet to 1.20.2",
						Node:        "cp3",
						Component:   kubernetes.Kubelet,
					},
					{
						MachineID:   "node-worker-1",
						Description: "worker1: updating kubelet to 1.20.2",
						Node:        "worker1",
						Component:   kubernetes.Kubelet,
					},
				},
			},
		},
		{
			name: "upgrade plan with locked nodes",
			machineMap: &kubernetes.MachineMap{
				ControlPlanes: map[string]string{
					"cp1": "node-cp-1",
				},
				Workers: map[string]string{
					"worker1": "node-worker-1",
					"worker2": "node-worker-2",
					"worker3": "node-worker-3",
				},
				Locked: map[string]string{
					"worker1": "node-worker-1",
					"worker3": "node-worker-3",
				},
			},
			kubernetesStatus: kubernetesStatus("cluster5", &specs.KubernetesStatusSpec{
				Nodes: []*specs.KubernetesStatusSpec_NodeStatus{
					{
						Nodename:       "cp1",
						KubeletVersion: "1.33.2",
						Ready:          true,
					},
					{
						Nodename:       "worker1",
						KubeletVersion: "1.33.1",
						Ready:          true,
					},
					{
						Nodename:       "worker2",
						KubeletVersion: "1.33.1",
						Ready:          true,
					},
					{
						Nodename:       "worker3",
						KubeletVersion: "1.33.1",
						Ready:          true,
					},
				},
				StaticPods: []*specs.KubernetesStatusSpec_NodeStaticPods{
					{
						Nodename: "cp1",
						StaticPods: []*specs.KubernetesStatusSpec_StaticPodStatus{
							{
								App:     "kube-apiserver",
								Version: "1.33.2",
								Ready:   true,
							},
							{
								App:     "kube-controller-manager",
								Version: "1.33.2",
								Ready:   true,
							},
							{
								App:     "kube-scheduler",
								Version: "1.33.2",
								Ready:   true,
							},
						},
					},
				},
			}),
			desiredVersion: "1.33.2",
			expected: &kubernetes.UpgradePath{
				ClusterID:          "cluster5",
				AllComponentsReady: true,
				AllNodes:           []string{"cp1", "worker1", "worker2", "worker3"},
				AllNodesToRequiredImages: map[string][]string{
					"cp1":     requiredImages("1.33.2", true),
					"worker1": requiredImages("1.33.2", false),
					"worker2": requiredImages("1.33.2", false),
					"worker3": requiredImages("1.33.2", false),
				},
				Steps: []kubernetes.UpgradeStep{
					{
						MachineID:   "node-worker-2",
						Description: "worker2: updating kubelet to 1.33.2",
						Node:        "worker2",
						Component:   kubernetes.Kubelet,
					},
					{
						MachineID:   "node-worker-1",
						Description: "worker1: updating kubelet to 1.33.2",
						Node:        "worker1",
						Component:   kubernetes.Kubelet,
						Blocked:     true,
					},
					{
						MachineID:   "node-worker-3",
						Description: "worker3: updating kubelet to 1.33.2",
						Node:        "worker3",
						Component:   kubernetes.Kubelet,
						Blocked:     true,
					},
				},
			},
		},
		{
			name: "upgrade plan waiting for locked nodes to be unlocked",
			machineMap: &kubernetes.MachineMap{
				ControlPlanes: map[string]string{
					"cp1": "node-cp-1",
				},
				Workers: map[string]string{
					"worker1": "node-worker-1",
					"worker2": "node-worker-2",
					"worker3": "node-worker-3",
				},
				Locked: map[string]string{
					"worker1": "node-worker-1",
					"worker3": "node-worker-3",
				},
			},
			kubernetesStatus: kubernetesStatus("cluster5", &specs.KubernetesStatusSpec{
				Nodes: []*specs.KubernetesStatusSpec_NodeStatus{
					{
						Nodename:       "cp1",
						KubeletVersion: "1.33.2",
						Ready:          true,
					},
					{
						Nodename:       "worker1",
						KubeletVersion: "1.33.1",
						Ready:          true,
					},
					{
						Nodename:       "worker2",
						KubeletVersion: "1.33.2",
						Ready:          true,
					},
					{
						Nodename:       "worker3",
						KubeletVersion: "1.33.1",
						Ready:          true,
					},
				},
				StaticPods: []*specs.KubernetesStatusSpec_NodeStaticPods{
					{
						Nodename: "cp1",
						StaticPods: []*specs.KubernetesStatusSpec_StaticPodStatus{
							{
								App:     "kube-apiserver",
								Version: "1.33.2",
								Ready:   true,
							},
							{
								App:     "kube-controller-manager",
								Version: "1.33.2",
								Ready:   true,
							},
							{
								App:     "kube-scheduler",
								Version: "1.33.2",
								Ready:   true,
							},
						},
					},
				},
			}),
			desiredVersion: "1.33.2",
			expected: &kubernetes.UpgradePath{
				ClusterID:          "cluster5",
				AllComponentsReady: true,
				AllNodes:           []string{"cp1", "worker1", "worker2", "worker3"},
				AllNodesToRequiredImages: map[string][]string{
					"cp1":     requiredImages("1.33.2", true),
					"worker1": requiredImages("1.33.2", false),
					"worker2": requiredImages("1.33.2", false),
					"worker3": requiredImages("1.33.2", false),
				},
				Steps: []kubernetes.UpgradeStep{
					{
						MachineID:   "node-worker-1",
						Description: "worker1: updating kubelet to 1.33.2",
						Node:        "worker1",
						Component:   kubernetes.Kubelet,
						Blocked:     true,
					},
					{
						MachineID:   "node-worker-3",
						Description: "worker3: updating kubelet to 1.33.2",
						Node:        "worker3",
						Component:   kubernetes.Kubelet,
						Blocked:     true,
					},
				},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			upgradePath := kubernetes.CalculateUpgradePath(test.machineMap, test.kubernetesStatus, test.desiredVersion)
			assert.Equal(t, test.expected.AllComponentsReady, upgradePath.AllComponentsReady)
			assert.Equal(t, test.expected.NotReadyStatus, upgradePath.NotReadyStatus)
			assert.Equal(t, test.expected.BlockedNodes(), upgradePath.BlockedNodes())

			require.Len(t, upgradePath.Steps, len(test.expected.Steps))

			for i := range upgradePath.Steps {
				assert.Equal(t, test.expected.Steps[i].Component, upgradePath.Steps[i].Component)
				assert.Equal(t, test.expected.Steps[i].Node, upgradePath.Steps[i].Node)
				assert.Equal(t, test.expected.Steps[i].MachineID, upgradePath.Steps[i].MachineID)
				assert.Equal(t, test.expected.Steps[i].Description, upgradePath.Steps[i].Description)
			}

			// assert the required images
			assert.Equal(t, test.expected.AllNodes, upgradePath.AllNodes)
			assert.Equal(t, test.expected.AllNodesToRequiredImages, upgradePath.AllNodesToRequiredImages)
		})
	}
}
