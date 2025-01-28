// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/cleanup"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime"
	"github.com/siderolabs/omni/internal/backend/runtime/kubernetes"
)

// ClusterController manages Cluster resource lifecycle.
//
// ClusterController plays the role of machine discovery.
type ClusterController = cleanup.Controller[*omni.Cluster]

type handler struct {
	handler   cleanup.Handler[*omni.Cluster]
	finalizer string
}

func (c handler) FinalizerRemoval(ctx context.Context, r controller.Runtime, logger *zap.Logger, input *omni.Cluster) error {
	if err := c.handler.FinalizerRemoval(ctx, r, logger, input); err != nil {
		return err
	}

	type kubeRuntime interface {
		DestroyClient(string)
	}

	// destroy client cache for the destroyed cluster
	k8s, err := runtime.LookupInterface[kubeRuntime](kubernetes.Name)
	if err != nil {
		return err
	}

	k8s.DestroyClient(input.Metadata().ID())

	return nil
}

func (c handler) Inputs() []controller.Input {
	return c.handler.Inputs()
}

func (c handler) Outputs() []controller.Output {
	return c.handler.Outputs()
}

// NewClusterController creates new ClusterController.
func NewClusterController() *ClusterController {
	name := "ClusterController"

	return cleanup.NewController(
		cleanup.Settings[*omni.Cluster]{
			Name: name,
			Handler: handler{
				finalizer: name,
				handler: cleanup.Combine(
					cleanup.RemoveOutputs[*omni.MachineSet](func(cluster *omni.Cluster) state.ListOption {
						return state.WithLabelQuery(
							resource.LabelEqual(omni.LabelCluster, cluster.Metadata().ID()),
							resource.LabelExists(omni.LabelWorkerRole),
						)
					}),
					cleanup.RemoveOutputs[*omni.MachineSet](func(cluster *omni.Cluster) state.ListOption {
						return state.WithLabelQuery(
							resource.LabelEqual(omni.LabelCluster, cluster.Metadata().ID()),
							resource.LabelExists(omni.LabelControlPlaneRole),
						)
					}),
					cleanup.RemoveOutputs[*omni.ConfigPatch](func(cluster *omni.Cluster) state.ListOption {
						return state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, cluster.Metadata().ID()))
					}),
					cleanup.RemoveOutputs[*omni.ExtensionsConfiguration](func(cluster *omni.Cluster) state.ListOption {
						return state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, cluster.Metadata().ID()))
					}),
					cleanup.HasNoOutputs[*omni.ClusterMachine](func(cluster *omni.Cluster) state.ListOption {
						return state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, cluster.Metadata().ID()))
					}),
					cleanup.HasNoOutputs[*omni.ClusterMachineConfigPatches](func(cluster *omni.Cluster) state.ListOption {
						return state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, cluster.Metadata().ID()))
					}),
					cleanup.HasNoOutputs[*omni.ClusterMachineConfig](func(cluster *omni.Cluster) state.ListOption {
						return state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, cluster.Metadata().ID()))
					}),
				),
			},
		},
	)
}
