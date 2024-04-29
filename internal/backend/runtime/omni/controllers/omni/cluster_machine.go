// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller/generic/cleanup"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// ClusterMachineController manages ClusterMachine resource lifecycle.
type ClusterMachineController = cleanup.Controller[*omni.ClusterMachine]

// NewClusterMachineController returns a new ClusterMachine controller.
func NewClusterMachineController() *ClusterMachineController {
	return cleanup.NewController(
		cleanup.Settings[*omni.ClusterMachine]{
			Name: "ClusterMachineController",
			Handler: cleanup.Combine(
				cleanup.RemoveOutputs[*omni.ExtensionsConfiguration](func(clusterMachine *omni.ClusterMachine) state.ListOption {
					clusterName, _ := clusterMachine.Metadata().Labels().Get(omni.LabelCluster)

					return state.WithLabelQuery(
						resource.LabelEqual(omni.LabelCluster, clusterName),
						resource.LabelEqual(omni.LabelClusterMachine, clusterMachine.Metadata().ID()),
					)
				}),
				withFinalizerCheck(
					cleanup.RemoveOutputs[*omni.ConfigPatch](func(clusterMachine *omni.ClusterMachine) state.ListOption {
						clusterName, _ := clusterMachine.Metadata().Labels().Get(omni.LabelCluster)

						return state.WithLabelQuery(
							resource.LabelEqual(omni.LabelCluster, clusterName),
							resource.LabelEqual(omni.LabelClusterMachine, clusterMachine.Metadata().ID()),
						)
					},
						cleanup.WithExtraOwners(KubernetesUpgradeStatusControllerName),
					), func(clusterMachine *omni.ClusterMachine) error {
						_, ok := clusterMachine.Metadata().Labels().Get(omni.LabelCluster)
						if !ok {
							return fmt.Errorf("cluster machine doesn't have %q label", omni.LabelCluster)
						}

						return nil
					}),
			),
		},
	)
}
