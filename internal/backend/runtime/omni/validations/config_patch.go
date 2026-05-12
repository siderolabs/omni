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

func configPatchValidationOptions(st state.State) []validated.StateOption {
	return []validated.StateOption{
		validated.WithCreateValidations(validated.NewCreateValidationForType(func(ctx context.Context, res *omni.ConfigPatch, _ ...state.CreateOption) error {
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

			buffer, err := res.TypedSpec().Value.GetUncompressedData()
			if err != nil {
				return err
			}

			defer buffer.Free()

			return omni.ValidateConfigPatch(buffer.Data())
		})),
		validated.WithUpdateValidations(validated.NewUpdateValidationForType(func(_ context.Context, oldRes *omni.ConfigPatch, newRes *omni.ConfigPatch, _ ...state.UpdateOption) error {
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
