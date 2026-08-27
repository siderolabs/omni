// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package cluster

import (
	"context"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/cleanup"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	customcleanup "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/cleanup"
)

// CleanupController removes the resources associated with a cluster when the cluster is destroyed.
type CleanupController = cleanup.Controller[*omni.Cluster]

// CleanupControllerName is the name of the cluster cleanup controller.
//
// The value keeps the old name of the controller, as it is persisted in the state, e.g., in the finalizers.
const CleanupControllerName = "ClusterController"

// KubernetesRuntime is the subset of the Kubernetes runtime the cluster cleanup needs.
type KubernetesRuntime interface {
	DestroyClient(string)
}

type handler struct {
	handler           cleanup.Handler[*omni.Cluster]
	kubernetesRuntime KubernetesRuntime
	finalizer         string
}

func (c handler) FinalizerRemoval(ctx context.Context, r controller.Runtime, logger *zap.Logger, input *omni.Cluster) error {
	if err := c.handler.FinalizerRemoval(ctx, r, logger, input); err != nil {
		return err
	}

	// destroy client cache for the destroyed cluster
	c.kubernetesRuntime.DestroyClient(input.Metadata().ID())

	return nil
}

func (c handler) Inputs() []controller.Input {
	return c.handler.Inputs()
}

func (c handler) Outputs() []controller.Output {
	return c.handler.Outputs()
}

// NewCleanupController creates a new cluster cleanup controller.
func NewCleanupController(kubernetesRuntime KubernetesRuntime) *CleanupController {
	return cleanup.NewController(
		cleanup.Settings[*omni.Cluster]{
			Name: CleanupControllerName,
			Handler: handler{
				kubernetesRuntime: kubernetesRuntime,
				finalizer:         CleanupControllerName,
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
					cleanup.RemoveOutputs[*omni.KubernetesManifestGroup](func(cluster *omni.Cluster) state.ListOption {
						return state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, cluster.Metadata().ID()))
					}),
					cleanup.RemoveOutputs[*omni.ExtensionsConfiguration](func(cluster *omni.Cluster) state.ListOption {
						return state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, cluster.Metadata().ID()))
					}),
					cleanup.RemoveOutputs[*omni.KubernetesHealthCheck](func(cluster *omni.Cluster) state.ListOption {
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
					&customcleanup.SameIDHandler[*omni.Cluster, *omni.ImportedClusterSecrets]{},
					&customcleanup.SameIDHandler[*omni.Cluster, *omni.RotateTalosCA]{},
					&customcleanup.SameIDHandler[*omni.Cluster, *omni.RotateKubernetesCA]{},
				),
			},
		},
	)
}
