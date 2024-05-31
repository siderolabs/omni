// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package machinestatus

import (
	"context"
	"fmt"
	"net/netip"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/channel"
	"github.com/siderolabs/siderolink/pkg/events"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/pkg/auth/actor"
)

// Handler is a machine status handler.
type Handler struct {
	logger   *zap.Logger
	state    state.State
	notifyCh chan<- *omni.MachineStatusSnapshot
}

// NewHandler creates a new machine status handler.
func NewHandler(state state.State, logger *zap.Logger, notifyCh chan<- *omni.MachineStatusSnapshot) *Handler {
	return &Handler{
		state:    state,
		logger:   logger,
		notifyCh: notifyCh,
	}
}

// HandleEvent is called on each event coming from the Talos nodes.
func (handler *Handler) HandleEvent(ctx context.Context, event events.Event) error {
	ctx = actor.MarkContextAsInternalActor(ctx)

	ipPort, err := netip.ParseAddrPort(event.Node)
	if err != nil {
		return err
	}

	ip := ipPort.Addr().String()

	machines, err := safe.StateListAll[*omni.Machine](
		ctx,
		handler.state,
		state.WithLabelQuery(resource.LabelEqual(omni.MachineAddressLabel, ip)),
	)
	if err != nil {
		return err
	}

	if machines.Len() == 0 {
		return fmt.Errorf("no machines found for address %s", ip)
	}

	switch event := event.Payload.(type) {
	case *machineapi.MachineStatusEvent:
		return handler.handleMachineStatusEvent(ctx, event, machines.Get(0).Metadata().ID())
	default: // nothing, we ignore other events
	}

	return nil
}

func (handler *Handler) handleMachineStatusEvent(ctx context.Context, event *machineapi.MachineStatusEvent, machineID resource.ID) error {
	handler.logger.Info("got machine status event",
		zap.String("machine", machineID),
		zap.String("stage", event.Stage.String()),
		zap.Any("status", event.Status),
	)

	snapshot := omni.NewMachineStatusSnapshot(resources.DefaultNamespace, machineID)

	snapshot.TypedSpec().Value.MachineStatus = event

	channel.SendWithContext(ctx, handler.notifyCh, snapshot)

	return nil
}
