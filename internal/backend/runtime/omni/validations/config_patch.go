// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package validations

import (
	"bytes"
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/validated"
)

// validateConfigPatchLevel checks that a config patch targets a single level.
//
// The machine label applies the patch in every cluster, so it stands alone. The machine set and
// cluster machine labels narrow a cluster down, so they require the cluster label and exclude each
// other.
func validateConfigPatchLevel(res *omni.ConfigPatch) error {
	labels := res.Metadata().Labels()

	_, cluster := labels.Get(omni.LabelCluster)
	_, machine := labels.Get(omni.LabelMachine)
	_, machineSet := labels.Get(omni.LabelMachineSet)
	_, clusterMachine := labels.Get(omni.LabelClusterMachine)

	if machine && (cluster || machineSet || clusterMachine) {
		return fmt.Errorf("label %q applies the patch to the machine in every cluster, so it cannot be combined with %q, %q or %q",
			omni.LabelMachine, omni.LabelCluster, omni.LabelMachineSet, omni.LabelClusterMachine)
	}

	if machineSet && clusterMachine {
		return fmt.Errorf("labels %q and %q cannot be set together, a patch narrows a cluster down in one way",
			omni.LabelMachineSet, omni.LabelClusterMachine)
	}

	if (machineSet || clusterMachine) && !cluster {
		return fmt.Errorf("label %q is required alongside %q and %q, the patch matches nothing without it",
			omni.LabelCluster, omni.LabelMachineSet, omni.LabelClusterMachine)
	}

	return nil
}

// validateConfigPatchTargetsRunning checks that the cluster and the machine set a patch targets are not tearing down.
func validateConfigPatchTargetsRunning(ctx context.Context, st state.State, res *omni.ConfigPatch) error {
	if clusterName, ok := res.Metadata().Labels().Get(omni.LabelCluster); ok {
		cluster, err := safe.StateGetByID[*omni.Cluster](ctx, st, clusterName)
		if err != nil && !state.IsNotFoundError(err) {
			return err
		}

		if cluster != nil && cluster.Metadata().Phase() == resource.PhaseTearingDown {
			return fmt.Errorf("cluster %q is tearing down", clusterName)
		}
	}

	if machineSetName, ok := res.Metadata().Labels().Get(omni.LabelMachineSet); ok {
		machineSet, err := safe.StateGetByID[*omni.MachineSet](ctx, st, machineSetName)
		if err != nil && !state.IsNotFoundError(err) {
			return err
		}

		if machineSet != nil && machineSet.Metadata().Phase() == resource.PhaseTearingDown {
			return fmt.Errorf("machine set %q is tearing down", machineSetName)
		}
	}

	return nil
}

func configPatchValidationOptions(st state.State) []validated.StateOption {
	return []validated.StateOption{
		validated.WithCreateValidations(validated.NewCreateValidationForType(func(ctx context.Context, res *omni.ConfigPatch, _ ...state.CreateOption) error {
			if err := validateConfigPatchLevel(res); err != nil {
				return err
			}

			if err := validateConfigPatchTargetsRunning(ctx, st, res); err != nil {
				return err
			}

			buffer, err := res.TypedSpec().Value.GetUncompressedData()
			if err != nil {
				return err
			}

			defer buffer.Free()

			return omni.ValidateConfigPatch(buffer.Data())
		})),
		validated.WithUpdateValidations(validated.NewUpdateValidationForType(func(_ context.Context, oldRes *omni.ConfigPatch, newRes *omni.ConfigPatch, _ ...state.UpdateOption) error {
			// before the unchanged data shortcut below, so a labels-only update is checked too
			if err := validateConfigPatchLevel(newRes); err != nil {
				return err
			}

			// keep the old config patch if the data is the same for backwards-compatibility and for teardown cases
			oldBuffer, err := oldRes.TypedSpec().Value.GetUncompressedData()
			if err != nil {
				return err
			}

			defer oldBuffer.Free()

			newBuffer, err := newRes.TypedSpec().Value.GetUncompressedData()
			if err != nil {
				return err
			}

			defer newBuffer.Free()

			oldData := oldBuffer.Data()
			newData := newBuffer.Data()

			if bytes.Equal(oldData, newData) {
				return nil
			}

			return omni.ValidateConfigPatch(newData)
		})),
	}
}
