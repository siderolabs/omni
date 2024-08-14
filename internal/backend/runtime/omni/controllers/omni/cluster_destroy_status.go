// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/gertd/go-pluralize"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// ClusterDestroyStatusController manages ClusterStatus resource lifecycle.
//
// ClusterDestroyStatusController aggregates the cluster state based on the cluster machines states.
type ClusterDestroyStatusController struct{}

// Name implements controller.Controller interface.
func (ctrl *ClusterDestroyStatusController) Name() string {
	return "ClusterDestroyStatusController"
}

// Inputs implements controller.Controller interface.
func (ctrl *ClusterDestroyStatusController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Type:      omni.ClusterType,
			Kind:      controller.InputDestroyReady,
			Namespace: resources.DefaultNamespace,
		},
		{
			Type:      omni.MachineSetStatusType,
			Kind:      controller.InputWeak,
			Namespace: resources.DefaultNamespace,
		},
		{
			Type:      omni.ClusterMachineStatusType,
			Kind:      controller.InputWeak,
			Namespace: resources.DefaultNamespace,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *ClusterDestroyStatusController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: omni.ClusterDestroyStatusType,
			Kind: controller.OutputExclusive,
		},
		{
			Type: omni.ClusterType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:dupl
func (ctrl *ClusterDestroyStatusController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		tracker := trackResource(r, resources.DefaultNamespace, omni.ClusterDestroyStatusType)

		clusters, err := safe.ReaderListAll[*omni.Cluster](ctx, r)
		if err != nil {
			return fmt.Errorf("error listing Cluster resources: %w", err)
		}

		for cluster := range clusters.All() {
			if cluster.Metadata().Phase() != resource.PhaseTearingDown {
				continue
			}

			tracker.keep(cluster)

			if err = ctrl.collectDestroyStatus(ctx, cluster, r); err != nil {
				return err
			}

			if cluster.Metadata().Finalizers().Empty() {
				if err = r.Destroy(ctx, cluster.Metadata(), controller.WithOwner("")); err != nil {
					if state.IsNotFoundError(err) {
						continue
					}

					return fmt.Errorf("failed to destroy cluster %w", err)
				}
			}
		}

		if err = tracker.cleanup(ctx); err != nil {
			return err
		}
	}
}

func (ctrl *ClusterDestroyStatusController) collectDestroyStatus(ctx context.Context, cluster *omni.Cluster, r controller.Runtime) error {
	var err error

	machineSets, err := r.List(ctx, omni.NewMachineSetStatus(resources.DefaultNamespace, "").Metadata(), state.WithLabelQuery(
		resource.LabelEqual(omni.LabelCluster, cluster.Metadata().ID()),
	))
	if err != nil {
		return fmt.Errorf("failed to list control planes %w", err)
	}

	remainingMachines := 0
	remainingMachineSets := len(machineSets.Items)

	for _, machineSet := range machineSets.Items {
		machines, err := r.List(ctx, omni.NewClusterMachineStatus(resources.DefaultNamespace, "").Metadata(),
			state.WithLabelQuery(resource.LabelEqual(
				omni.LabelMachineSet, machineSet.Metadata().ID()),
			),
		)
		if err != nil {
			return err
		}

		remainingMachines += len(machines.Items)
	}

	return safe.WriterModify(ctx, r, omni.NewClusterDestroyStatus(resources.DefaultNamespace, cluster.Metadata().ID()), func(status *omni.ClusterDestroyStatus) error {
		status.TypedSpec().Value.Phase = fmt.Sprintf("Destroying: %s, %s",
			pluralize.NewClient().Pluralize("machine set", remainingMachineSets, true),
			pluralize.NewClient().Pluralize("machine", remainingMachines, true),
		)

		return nil
	})
}
