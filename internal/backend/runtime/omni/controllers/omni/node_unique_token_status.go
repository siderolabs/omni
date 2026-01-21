// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/google/uuid"
	"github.com/siderolabs/gen/xerrors"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/jointoken"
	"github.com/siderolabs/omni/client/pkg/meta"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	siderolinkres "github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/client/pkg/omni/resources/system"
	"github.com/siderolabs/omni/client/pkg/siderolink"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/mappers"
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
			TransformFunc: handler.reconcileRunning,
		},
		qtransform.WithExtraMappedInput[*siderolinkres.NodeUniqueToken](qtransform.MapperSameID[*machineStatusLabels]()),
		qtransform.WithExtraMappedInput[*omni.ClusterMachine](qtransform.MapperSameID[*machineStatusLabels]()),
		qtransform.WithExtraMappedInput[*omni.MachineStatusSnapshot](qtransform.MapperSameID[*machineStatusLabels]()),
		qtransform.WithExtraMappedInput[*omni.Machine](qtransform.MapperSameID[*machineStatusLabels]()),
		qtransform.WithExtraMappedInput[*omni.TalosConfig](mappers.MapClusterResourceToLabeledResources[*machineStatusLabels]()),
		qtransform.WithConcurrency(32),
	)
}

type nodeUniqueTokenStatusHandler struct{}

func (handler *nodeUniqueTokenStatusHandler) reconcileRunning(
	ctx context.Context,
	r controller.Reader,
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

	c, err := handler.getClient(ctx, r, machine)
	if err != nil {
		return err
	}

	defer c.Close() //nolint:errcheck

	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	logger.Info("proactively generating node unique token")

	return handler.generateUniqueNodeToken(ctx, c, logger)
}

func (handler *nodeUniqueTokenStatusHandler) generateUniqueNodeToken(
	ctx context.Context,
	c *client.Client,
	logger *zap.Logger,
) error {
	uniqueToken, err := safe.ReaderGetByID[*runtime.MetaKey](ctx, c.COSI, runtime.MetaKeyTagToID(meta.UniqueMachineToken))
	if err != nil && !state.IsNotFoundError(err) {
		return err
	}

	if uniqueToken != nil {
		logger.Debug("meta key already exists")

		return nil
	}

	fingerprint, err := jointoken.GetMachineFingerprint(ctx, c)
	if err != nil {
		return err
	}

	token, err := jointoken.NewNodeUniqueToken(fingerprint, uuid.NewString()).Encode()
	if err != nil {
		return err
	}

	if err := c.MetaWrite(ctx, meta.UniqueMachineToken, []byte(token)); err != nil {
		return err
	}

	logger.Info("generated node unique secret token")

	return nil
}

func (handler *nodeUniqueTokenStatusHandler) getClient(
	ctx context.Context,
	r controller.Reader,
	machine *omni.Machine,
) (*client.Client, error) {
	if strings.HasPrefix(machine.TypedSpec().Value.ManagementAddress, "unix://") {
		return helpers.GetTalosClient[*omni.ClusterMachine](ctx, r, machine.TypedSpec().Value.ManagementAddress, nil)
	}

	clusterMachine, err := safe.ReaderGetByID[*omni.ClusterMachine](ctx, r, machine.Metadata().ID())
	if err != nil && !state.IsNotFoundError(err) {
		return nil, err
	}

	return helpers.GetTalosClient(ctx, r, machine.TypedSpec().Value.ManagementAddress, clusterMachine)
}
