// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"errors"
	"slices"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xerrors"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
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
				return omni.NewConfigPatch(resources.DefaultNamespace, request.Metadata().ID())
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

				if machineRequestStatus.TypedSpec().Value.Id == "" {
					return errors.New("failed to create config patch from the request: machine request status doesn't have machine UUID")
				}

				patch.TypedSpec().Value = request.TypedSpec().Value

				helpers.CopyAllLabels(request, patch)

				patch.Metadata().Labels().Set(omni.LabelSystemPatch, "")
				patch.Metadata().Labels().Set(omni.LabelMachine, machineRequestStatus.TypedSpec().Value.Id)

				return nil
			},
		},
		qtransform.WithExtraMappedInput(
			func(ctx context.Context, _ *zap.Logger, r controller.QRuntime, machineRequestStatus *infra.MachineRequestStatus) ([]resource.Pointer, error) {
				patchRequests, err := safe.ReaderListAll[*infra.ConfigPatchRequest](ctx, r, state.WithLabelQuery(
					resource.LabelEqual(omni.LabelMachineRequest, machineRequestStatus.Metadata().ID())),
				)
				if err != nil {
					return nil, err
				}

				return slices.Collect(patchRequests.Pointers()), nil
			},
		),
		qtransform.WithOutputKind(controller.OutputShared),
	)
}
