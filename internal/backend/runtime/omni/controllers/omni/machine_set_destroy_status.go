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

// MachineSetDestroyStatusController manages MachineSet resource lifecycle.
type MachineSetDestroyStatusController struct{}

// Name implements controller.Controller interface.
func (ctrl *MachineSetDestroyStatusController) Name() string {
	return "MachineSetDestroyStatusController"
}

// Inputs implements controller.Controller interface.
func (ctrl *MachineSetDestroyStatusController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Type:      omni.MachineSetType,
			Kind:      controller.InputDestroyReady,
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
func (ctrl *MachineSetDestroyStatusController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: omni.MachineSetDestroyStatusType,
			Kind: controller.OutputExclusive,
		},
		{
			Type: omni.MachineSetType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:dupl
func (ctrl *MachineSetDestroyStatusController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		tracker := trackResource(r, resources.EphemeralNamespace, omni.MachineSetDestroyStatusType)

		machineSets, err := safe.ReaderListAll[*omni.MachineSet](ctx, r)
		if err != nil {
			return fmt.Errorf("error listing Machine Set resources: %w", err)
		}

		for iter := machineSets.Iterator(); iter.Next(); {
			machineSet := iter.Value()

			if machineSet.Metadata().Phase() != resource.PhaseTearingDown {
				continue
			}

			tracker.keep(machineSet)

			if err = ctrl.collectDestroyStatus(ctx, machineSet, r); err != nil {
				return err
			}

			if machineSet.Metadata().Finalizers().Empty() {
				if err = r.Destroy(ctx, machineSet.Metadata(), controller.WithOwner("")); err != nil {
					if state.IsNotFoundError(err) {
						continue
					}

					return fmt.Errorf("failed to destroy machine set: %w", err)
				}
			}
		}

		if err = tracker.cleanup(ctx); err != nil {
			return err
		}
	}
}

func (ctrl *MachineSetDestroyStatusController) collectDestroyStatus(ctx context.Context, machineSet *omni.MachineSet, r controller.Runtime) error {
	var err error

	machines, err := r.List(ctx, omni.NewClusterMachineStatus(resources.DefaultNamespace, machineSet.Metadata().ID()).Metadata(),
		state.WithLabelQuery(resource.LabelEqual(
			omni.LabelMachineSet, machineSet.Metadata().ID()),
		),
	)
	if err != nil {
		return err
	}

	remainingMachines := len(machines.Items)

	return safe.WriterModify(ctx, r, omni.NewMachineSetDestroyStatus(resources.EphemeralNamespace, machineSet.Metadata().ID()), func(status *omni.MachineSetDestroyStatus) error {
		status.TypedSpec().Value.Phase = fmt.Sprintf("Destroying: %s",
			pluralize.NewClient().Pluralize("machine", remainingMachines, true),
		)

		return nil
	})
}
