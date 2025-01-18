// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package machineevent

import (
	"context"
	"fmt"
	"net/netip"
	"strings"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/channel"
	"github.com/siderolabs/siderolink/pkg/events"
	"github.com/siderolabs/talos/pkg/machinery/api/common"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/pkg/auth/actor"
)

// Handler is a machine event handler.
type Handler struct {
	logger         *zap.Logger
	state          state.State
	notifyCh       chan<- *omni.MachineStatusSnapshot
	installEventCh chan<- resource.ID
}

// NewHandler creates a new machine event handler.
func NewHandler(state state.State, logger *zap.Logger, notifyCh chan<- *omni.MachineStatusSnapshot, installEventCh chan<- resource.ID) *Handler {
	return &Handler{
		state:          state,
		logger:         logger,
		notifyCh:       notifyCh,
		installEventCh: installEventCh,
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

	machineID := machines.Get(0).Metadata().ID()

	switch event := event.Payload.(type) {
	case *machineapi.MachineStatusEvent:
		return handler.handleMachineStatusEvent(ctx, event, machineID)
	case *machineapi.SequenceEvent:
		return handler.handleSequenceEvent(ctx, event, machineID)
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

// handleSequenceEvent handles a sequence event: it updates the infra.MachineState if there is a matching infra.Machine.
func (handler *Handler) handleSequenceEvent(ctx context.Context, event *machineapi.SequenceEvent, machineID resource.ID) error {
	logger := handler.logger.With(zap.String("machine", machineID), zap.String("sequence", event.Sequence), zap.Stringer("action", event.Action))

	logger.Debug("got machine sequence event")

	// installation detection logic, taken from:
	// https://github.com/siderolabs/sidero/blob/999e6e9fae0419c5245f3807d000f1af90dc90ba/app/sidero-controller-manager/cmd/events-manager/adapter.go#L177-L196
	setInstalled := false

	switch {
	case event.GetSequence() == "install" && event.GetAction() == machineapi.SequenceEvent_NOOP && event.GetError().GetCode() == common.Code_FATAL:
		if strings.Contains(event.GetError().GetMessage(), "unix.Reboot") {
			setInstalled = true
		}
	case event.GetSequence() == "boot" && event.GetAction() == machineapi.SequenceEvent_START:
		setInstalled = true
	}

	if !setInstalled {
		return nil
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case handler.installEventCh <- machineID:
	}

	logger.Info("sent machine installed event", zap.String("id", machineID))

	return nil
}
