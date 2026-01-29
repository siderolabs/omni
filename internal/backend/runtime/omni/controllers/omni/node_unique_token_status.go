// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xerrors"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	siderolinkres "github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/client/pkg/omni/resources/system"
	"github.com/siderolabs/omni/client/pkg/siderolink"
)

// NodeUniqueTokenStatusController is a controller that writes the unique token to the meta partition the connected machines.
// Besides writing the unique tokens during migration, it generates the token status.
type NodeUniqueTokenStatusController = qtransform.QController[*machineStatusLabels, *siderolinkres.NodeUniqueTokenStatus]

// NewNodeUniqueTokenStatusController initializes NodeUniqueTokenStatusController.
func NewNodeUniqueTokenStatusController() *NodeUniqueTokenStatusController {
	handler := &nodeUniqueTokenStatusHandler{}

	return qtransform.NewQController(
		qtransform.Settings[*machineStatusLabels, *siderolinkres.NodeUniqueTokenStatus]{
			Name: "NodeUniqueTokenStatusController",
			MapMetadataFunc: func(machine *machineStatusLabels) *siderolinkres.NodeUniqueTokenStatus {
				return siderolinkres.NewNodeUniqueTokenStatus(machine.Metadata().ID())
			},
			UnmapMetadataFunc: func(nodeUniqueTokenStatus *siderolinkres.NodeUniqueTokenStatus) *machineStatusLabels {
				return system.NewResourceLabels[*omni.MachineStatus](nodeUniqueTokenStatus.Metadata().ID())
			},
			TransformExtraOutputFunc: handler.reconcileRunning,
		},
		qtransform.WithExtraMappedInput[*siderolinkres.Link](qtransform.MapperSameID[*machineStatusLabels]()),
		qtransform.WithExtraMappedInput[*siderolinkres.NodeUniqueToken](qtransform.MapperSameID[*machineStatusLabels]()),
		qtransform.WithExtraMappedInput[*omni.ClusterMachine](qtransform.MapperSameID[*machineStatusLabels]()),
		qtransform.WithExtraMappedInput[*omni.MachineStatusSnapshot](qtransform.MapperSameID[*machineStatusLabels]()),
		qtransform.WithExtraMappedInput[*omni.Machine](qtransform.MapperSameID[*machineStatusLabels]()),
		qtransform.WithExtraOutputs(controller.Output{
			Type: siderolinkres.PendingMachineType,
			Kind: controller.OutputShared,
		}),
		qtransform.WithConcurrency(32),
	)
}

type nodeUniqueTokenStatusHandler struct{}

func (handler *nodeUniqueTokenStatusHandler) reconcileRunning(
	ctx context.Context,
	r controller.ReaderWriter,
	logger *zap.Logger,
	machineStatus *machineStatusLabels,
	nodeUniqueTokenStatus *siderolinkres.NodeUniqueTokenStatus,
) error {
	talosVersion, ok := machineStatus.Metadata().Labels().Get(omni.MachineStatusLabelTalosVersion)

	if !ok {
		logger.Debug("the token is not generated: the machine version is unknown")

		return nil
	}

	if !siderolink.SupportsSecureJoinTokens(talosVersion) {
		logger.Debug("the token is not generated: not supported by the machine Talos version",
			zap.String("version", talosVersion),
		)

		nodeUniqueTokenStatus.TypedSpec().Value.State = specs.NodeUniqueTokenStatusSpec_UNSUPPORTED

		return nil
	}

	token, err := safe.ReaderGetByID[*siderolinkres.NodeUniqueToken](ctx, r, machineStatus.Metadata().ID())
	if err != nil && !state.IsNotFoundError(err) {
		return err
	}

	machine, err := safe.ReaderGetByID[*omni.Machine](ctx, r, machineStatus.Metadata().ID())
	if err != nil {
		return err
	}

	_, createdWithUniqueToken := machine.Metadata().Annotations().Get(omni.CreatedWithUniqueToken)
	if createdWithUniqueToken && token == nil {
		return xerrors.NewTaggedf[qtransform.SkipReconcileTag](
			"the machine was created with the unique token, but the resource doesn't exist yet",
		)
	}

	_, installed := machineStatus.Metadata().Labels().Get(omni.MachineStatusLabelInstalled)

	if token != nil && token.TypedSpec().Value.Token != "" {
		if installed {
			nodeUniqueTokenStatus.TypedSpec().Value.State = specs.NodeUniqueTokenStatusSpec_PERSISTENT
		} else {
			nodeUniqueTokenStatus.TypedSpec().Value.State = specs.NodeUniqueTokenStatusSpec_EPHEMERAL
		}

		// no need to do anything, the token is already generated
		return nil
	}

	nodeUniqueTokenStatus.TypedSpec().Value.State = specs.NodeUniqueTokenStatusSpec_NONE

	if _, connected := machineStatus.Metadata().Labels().Get(omni.MachineStatusLabelConnected); !connected {
		logger.Debug("the token is not generated: the machine is not connected")

		return nil
	}

	link, err := safe.ReaderGetByID[*siderolinkres.Link](ctx, r, machineStatus.Metadata().ID())
	if err != nil {
		return err
	}

	// create a pending machine status to force generating the node unique token
	return safe.WriterModify(ctx, r, siderolinkres.NewPendingMachine(link.TypedSpec().Value.NodePublicKey, link.TypedSpec().Value),
		func(pendingMachine *siderolinkres.PendingMachine) error {
			pendingMachine.Metadata().Labels().Set(omni.MachineUUID, link.Metadata().ID())

			pendingMachine.TypedSpec().Value = link.TypedSpec().Value

			return nil
		},
		controller.WithModifyNoOwner(),
	)
}
