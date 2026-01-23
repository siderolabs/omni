// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package clustermachine

import (
	"context"
	"errors"
	"slices"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xiter"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/configpatch"
)

type ConfigPatchesController struct {
	*qtransform.QController[*omni.ClusterMachine, *omni.ClusterMachineConfigPatches]
}

func NewConfigPatchesController() *ConfigPatchesController {
	ctrl := &ConfigPatchesController{}

	ctrl.QController = qtransform.NewQController(
		qtransform.Settings[*omni.ClusterMachine, *omni.ClusterMachineConfigPatches]{
			Name: "ClusterMachineConfigPatchesController",
			MapMetadataFunc: func(clusterMachine *omni.ClusterMachine) *omni.ClusterMachineConfigPatches {
				return omni.NewClusterMachineConfigPatches(clusterMachine.Metadata().ID())
			},
			UnmapMetadataFunc: func(clusterMachineConfigPatches *omni.ClusterMachineConfigPatches) *omni.ClusterMachine {
				return omni.NewClusterMachine(clusterMachineConfigPatches.Metadata().ID())
			},
			TransformFunc: ctrl.transform,
		},
		qtransform.WithExtraMappedInput[*omni.ConfigPatch](
			// config patch to cluster machine if the machine is allocated, checks by different layers
			// if is on the cluster layer, matches all machine sets
			func(ctx context.Context, _ *zap.Logger, r controller.QRuntime, patch controller.ReducedResourceMetadata) ([]resource.Pointer, error) {
				clusterName, ok := patch.Labels().Get(omni.LabelCluster)
				if !ok {
					// no cluster, map by the machine ID
					return mapToMachineID(ctx, r, patch, omni.LabelMachine)
				}

				// cluster machine patch
				pointers, err := mapToMachineID(ctx, r, patch, omni.LabelClusterMachine)
				if err != nil {
					return nil, err
				}

				if pointers != nil {
					return pointers, err
				}

				// machine set level patch
				machineSetID, ok := patch.Labels().Get(omni.LabelMachineSet)
				if ok {
					var res safe.List[*omni.ClusterMachine]

					res, err = safe.ReaderListAll[*omni.ClusterMachine](ctx, r, state.WithLabelQuery(resource.LabelEqual(omni.LabelMachineSet, machineSetID)))

					return slices.Collect(res.Pointers()), err
				}

				res, err := safe.ReaderListAll[*omni.ClusterMachine](ctx, r, state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, clusterName)))

				return slices.Collect(res.Pointers()), err
			},
		),
	)

	return ctrl
}

func (ctrl *ConfigPatchesController) transform(ctx context.Context, r controller.Reader, logger *zap.Logger,
	clusterMachine *omni.ClusterMachine, clusterMachineConfigPatches *omni.ClusterMachineConfigPatches,
) error {
	patchHelper, err := configpatch.NewHelper(ctx, r)
	if err != nil {
		return err
	}

	machineSetName, ok := clusterMachine.Metadata().Labels().Get(omni.LabelMachineSet)
	if !ok {
		return errors.New("cluster machine is missing machine set label")
	}

	configPatches, err := patchHelper.Get(clusterMachine, omni.NewMachineSet(machineSetName))
	if err != nil {
		return err
	}

	// update ClusterMachineConfigPatches resource with the list of matching patches for the machine
	// nothing changed in the patch list, skip any updates
	if !helpers.UpdateInputsVersions(clusterMachineConfigPatches, configPatches...) {
		return nil
	}

	helpers.CopyAllLabels(clusterMachine, clusterMachineConfigPatches)

	return setPatches(clusterMachineConfigPatches, configPatches)
}

func mapToMachineID(ctx context.Context, r controller.QRuntime, res controller.ReducedResourceMetadata, label string) ([]resource.Pointer, error) {
	id, ok := res.Labels().Get(label)
	if !ok {
		return nil, nil
	}

	input, err := safe.ReaderGetByID[*omni.ClusterMachine](ctx, r, id)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, nil
		}
	}

	return []resource.Pointer{
		input.Metadata(),
	}, nil
}

//nolint:staticcheck // we are ok with using the deprecated method here
func setPatches(clusterMachineConfigPatches *omni.ClusterMachineConfigPatches, patches []*omni.ConfigPatch) error {
	// clear the patch fields
	clusterMachineConfigPatches.TypedSpec().Value.Patches = nil
	clusterMachineConfigPatches.TypedSpec().Value.CompressedPatches = nil

	return clusterMachineConfigPatches.TypedSpec().Value.FromConfigPatches(
		xiter.Map(
			func(in *omni.ConfigPatch) *specs.ConfigPatchSpec { return in.TypedSpec().Value },
			slices.Values(patches),
		),
		specs.GetCompressionConfig(),
	)
}
