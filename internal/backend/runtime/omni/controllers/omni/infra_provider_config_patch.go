// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"slices"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xerrors"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
)

// InfraProviderConfigPatchController manages endpoints for each Cluster.
type InfraProviderConfigPatchController = qtransform.QController[*infra.ConfigPatchRequest, *omni.ConfigPatch]

// NewInfraProviderConfigPatchController initializes ConfigPatchRequestController.
func NewInfraProviderConfigPatchController() *InfraProviderConfigPatchController {
	return qtransform.NewQController(
		qtransform.Settings[*infra.ConfigPatchRequest, *omni.ConfigPatch]{
			Name: "ConfigPatchRequestController",
			MapMetadataFunc: func(request *infra.ConfigPatchRequest) *omni.ConfigPatch {
				return omni.NewConfigPatch(request.Metadata().ID())
			},
			UnmapMetadataFunc: func(configPatch *omni.ConfigPatch) *infra.ConfigPatchRequest {
				return infra.NewConfigPatchRequest(configPatch.Metadata().ID())
			},
			TransformFunc: func(ctx context.Context, r controller.Reader, _ *zap.Logger, request *infra.ConfigPatchRequest, patch *omni.ConfigPatch) error {
				machineRequestID, ok := request.Metadata().Labels().Get(omni.LabelMachineRequest)
				if !ok {
					return xerrors.NewTaggedf[qtransform.DestroyOutputTag]("missing machine request label on the patch request")
				}

				machineRequestStatus, err := safe.ReaderGetByID[*infra.MachineRequestStatus](ctx, r, machineRequestID)
				if err != nil {
					if state.IsNotFoundError(err) {
						return xerrors.NewTaggedf[qtransform.DestroyOutputTag]("machine request status with id %q doesn't exist", machineRequestID)
					}

					return err
				}

				if !sameInfraProvider(request, machineRequestStatus) {
					return xerrors.NewTaggedf[qtransform.DestroyOutputTag]("machine request %q is not of the infra provider of the patch request", machineRequestID)
				}

				// the UUID in the status is reported by the infra provider, it counts only when that machine is bound to the machine request
				machineID := machineRequestStatus.TypedSpec().Value.Id

				machineIDs, err := machinesOfRequest(ctx, r.List, machineRequestID)
				if err != nil {
					return err
				}

				if !slices.Contains(machineIDs, machineID) {
					return xerrors.NewTaggedf[qtransform.DestroyOutputTag]("the machine reported for the machine request %q is not bound to it", machineRequestID)
				}

				patch.TypedSpec().Value = request.TypedSpec().Value

				helpers.CopyAllLabels(request, patch)

				patch.Metadata().Labels().Set(omni.LabelSystemPatch, "")
				patch.Metadata().Labels().Set(omni.LabelMachine, machineID)

				return nil
			},
		},
		qtransform.WithExtraMappedInput[*infra.MachineRequestStatus](
			func(ctx context.Context, _ *zap.Logger, r controller.QRuntime, machineRequestStatus controller.ReducedResourceMetadata) ([]resource.Pointer, error) {
				return patchRequestsOfMachineRequest(ctx, r, machineRequestStatus.ID())
			},
		),
		// a link gets bound to a machine request after the patch request was created
		qtransform.WithExtraMappedInput[*siderolink.Link](
			func(ctx context.Context, _ *zap.Logger, r controller.QRuntime, link controller.ReducedResourceMetadata) ([]resource.Pointer, error) {
				machineRequestID, ok := link.Labels().Get(omni.LabelMachineRequest)
				if !ok {
					return nil, nil
				}

				return patchRequestsOfMachineRequest(ctx, r, machineRequestID)
			},
		),
		qtransform.WithOutputKind(controller.OutputShared),
	)
}

func patchRequestsOfMachineRequest(ctx context.Context, r controller.Reader, machineRequestID resource.ID) ([]resource.Pointer, error) {
	patchRequests, err := safe.ReaderListAll[*infra.ConfigPatchRequest](
		ctx, r, state.WithLabelQuery(
			resource.LabelEqual(omni.LabelMachineRequest, machineRequestID),
		),
	)
	if err != nil {
		return nil, err
	}

	return slices.Collect(patchRequests.Pointers()), nil
}
