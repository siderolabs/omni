// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"fmt"
	"net/netip"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/google/uuid"
	"github.com/siderolabs/gen/xerrors"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/meta"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
)

// PendingMachineStatusController is a controller that writes the meta key to the pending machine.
type PendingMachineStatusController = qtransform.QController[*siderolink.PendingMachine, *siderolink.PendingMachineStatus]

// NewPendingMachineStatusController initializes PendingMachineStatusController.
func NewPendingMachineStatusController() *PendingMachineStatusController {
	handler := &pendingMachineStatusHandler{}

	return qtransform.NewQController(
		qtransform.Settings[*siderolink.PendingMachine, *siderolink.PendingMachineStatus]{
			Name: "PendingMachineStatusController",
			MapMetadataFunc: func(pendingMachine *siderolink.PendingMachine) *siderolink.PendingMachineStatus {
				return siderolink.NewPendingMachineStatus(pendingMachine.Metadata().ID())
			},
			UnmapMetadataFunc: func(pendingMachineStatus *siderolink.PendingMachineStatus) *siderolink.PendingMachine {
				return siderolink.NewPendingMachine(pendingMachineStatus.Metadata().ID(), nil)
			},
			TransformFunc: handler.reconcileRunning,
		},
		qtransform.WithExtraMappedInput(func(_ context.Context, _ *zap.Logger, _ controller.QRuntime, res *siderolink.LinkStatus) ([]resource.Pointer, error) {
			return []resource.Pointer{
				siderolink.NewPendingMachine(res.TypedSpec().Value.LinkId, nil).Metadata(),
			}, nil
		}),
		qtransform.WithExtraMappedInput(qtransform.MapperNone[*siderolink.Link]()),
		qtransform.WithExtraMappedInput(qtransform.MapperNone[*omni.ClusterMachine]()),
		qtransform.WithExtraMappedInput(qtransform.MapperNone[*omni.TalosConfig]()),
		qtransform.WithConcurrency(32),
	)
}

type pendingMachineStatusHandler struct{}

func (handler *pendingMachineStatusHandler) reconcileRunning(
	ctx context.Context,
	r controller.Reader,
	logger *zap.Logger,
	pendingMachine *siderolink.PendingMachine,
	pendingMachineStatus *siderolink.PendingMachineStatus,
) error {
	_, err := safe.ReaderGet[*siderolink.LinkStatus](ctx, r, siderolink.NewLinkStatus(pendingMachine).Metadata())
	if err != nil {
		if state.IsNotFoundError(err) {
			return xerrors.NewTagged[qtransform.SkipReconcileTag](err)
		}

		return err
	}

	c, err := getClient(ctx, r, pendingMachine)
	if err != nil {
		return err
	}

	defer c.Close() //nolint:errcheck

	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	if err = handler.handleUUIDConflicts(ctx, r, c, logger, pendingMachine, pendingMachineStatus); err != nil {
		return err
	}

	return handler.generateUniqueNodeToken(ctx, c, logger, pendingMachineStatus)
}

func (handler *pendingMachineStatusHandler) generateUniqueNodeToken(
	ctx context.Context,
	c *client.Client,
	logger *zap.Logger,
	pendingMachineStatus *siderolink.PendingMachineStatus,
) error {
	// skip any other attempts to write the token if it was already written
	if pendingMachineStatus.TypedSpec().Value.Token != "" {
		return nil
	}

	token := uuid.NewString()

	if err := c.MetaWrite(ctx, meta.UniqueMachineToken, []byte(token)); err != nil {
		return err
	}

	pendingMachineStatus.TypedSpec().Value.Token = token

	logger.Info("generated node unique secret token")

	return nil
}

func (handler *pendingMachineStatusHandler) handleUUIDConflicts(
	ctx context.Context,
	r controller.Reader,
	c *client.Client,
	logger *zap.Logger,
	pendingMachine *siderolink.PendingMachine,
	pendingMachineStatus *siderolink.PendingMachineStatus,
) error {
	// skip if the machine UUID was already configured
	_, configured := pendingMachineStatus.Metadata().Annotations().Get(omni.MachineUUID)
	if configured {
		return nil
	}

	machineUUID, ok := pendingMachine.Metadata().Labels().Get(omni.MachineUUID)
	if !ok {
		return fmt.Errorf("machine UUID is not set on the pending machine")
	}

	conflict, err := detectUUIDConflict(ctx, r, pendingMachine, machineUUID)
	if err != nil {
		return err
	}

	if !conflict {
		pendingMachineStatus.Metadata().Annotations().Set(omni.MachineUUID, machineUUID)

		return nil
	}

	id := uuid.NewString()

	if err = c.MetaWrite(ctx, meta.UUIDOverride, []byte(id)); err != nil {
		return err
	}

	logger.Info("detected UUID conflict, generated a random ID for the node", zap.String("machine", machineUUID), zap.String("new_uuid", id))

	pendingMachineStatus.Metadata().Annotations().Set(omni.MachineUUID, id)

	return nil
}

func detectUUIDConflict(ctx context.Context, r controller.Reader, pendingMachine *siderolink.PendingMachine, machineUUID string) (bool, error) {
	conflicts, err := safe.ReaderListAll[*siderolink.PendingMachine](ctx, r, state.WithLabelQuery(
		resource.LabelEqual(omni.MachineUUID, machineUUID),
	))
	if err != nil {
		return false, err
	}

	if _, conflictFound := conflicts.Find(func(r *siderolink.PendingMachine) bool {
		return r.Metadata().ID() != pendingMachine.Metadata().ID()
	}); conflictFound {
		return true, nil
	}

	link, err := safe.ReaderGetByID[*siderolink.Link](ctx, r, machineUUID)
	if err != nil && !state.IsNotFoundError(err) {
		return false, err
	}

	return link != nil && link.TypedSpec().Value.NodeUniqueToken != "", nil
}

func getClient(
	ctx context.Context,
	r controller.Reader,
	pendingMachine *siderolink.PendingMachine,
) (*client.Client, error) {
	if strings.HasPrefix(pendingMachine.TypedSpec().Value.NodeSubnet, "unix://") {
		return helpers.GetTalosClient[*omni.ClusterMachine](ctx, r, pendingMachine.TypedSpec().Value.NodeSubnet, nil)
	}

	ipPrefix, err := netip.ParsePrefix(pendingMachine.TypedSpec().Value.NodeSubnet)
	if err != nil {
		return nil, err
	}

	address := ipPrefix.Addr().String()

	machineUUID, ok := pendingMachine.Metadata().Labels().Get(omni.MachineUUID)
	if !ok {
		return nil, fmt.Errorf("machine UUID is not set on the pending machine")
	}

	clusterMachine, err := safe.ReaderGetByID[*omni.ClusterMachine](ctx, r, machineUUID)
	if err != nil && !state.IsNotFoundError(err) {
		return nil, err
	}

	return helpers.GetTalosClient(ctx, r, address, clusterMachine)
}
