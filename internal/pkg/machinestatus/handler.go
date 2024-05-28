// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package machinestatus

import (
	"context"
	"fmt"
	"net/netip"
	"sync"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/siderolink/pkg/events"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/pkg/auth/actor"
)

// Clock is here to be able to mock time.Now() in the tests.
type Clock interface {
	Now() time.Time
}

type clock struct{}

func (c *clock) Now() time.Time {
	return time.Now()
}

type eventInfo struct {
	timestamp      time.Time
	fromSideroLink bool
}

// Handler is a machine status handler.
type Handler struct {
	logger *zap.Logger
	state  state.State
	Clock  Clock

	machineToLastEventInfo map[resource.ID]eventInfo

	lock sync.Mutex
}

// NewHandler creates a new machine status handler.
func NewHandler(state state.State, logger *zap.Logger) *Handler {
	return &Handler{
		state:                  state,
		logger:                 logger,
		machineToLastEventInfo: make(map[resource.ID]eventInfo),
		Clock:                  &clock{},
	}
}

// Start starts the machine status handler.
func (handler *Handler) Start(ctx context.Context) error {
	eventCh := make(chan state.Event)

	machineKind := omni.NewMachine(resources.DefaultNamespace, "").Metadata()
	machineStatusKind := omni.NewMachineStatus(resources.DefaultNamespace, "").Metadata()

	if err := handler.state.WatchKind(ctx, machineStatusKind, eventCh); err != nil {
		return fmt.Errorf("error watching machine statuses: %w", err)
	}

	if err := handler.state.WatchKind(ctx, machineKind, eventCh); err != nil {
		return fmt.Errorf("error watching machines: %w", err)
	}

	for {
		var event state.Event

		select {
		case <-ctx.Done():
			return nil
		case event = <-eventCh:
			switch event.Type {
			case state.Errored:
				return fmt.Errorf("error watching resources: %w", event.Error)
			case state.Bootstrapped: // ignore
			case state.Created, state.Updated:
				if machineStatus, ok := event.Resource.(*omni.MachineStatus); ok {
					handler.handleMachineStatusResourceUpdate(ctx, machineStatus)
				}
			case state.Destroyed:
				if machine, ok := event.Resource.(*omni.Machine); ok {
					handler.handleMachineDestroy(ctx, machine)
				}
			}
		}
	}
}

func (handler *Handler) handleMachineStatusResourceUpdate(ctx context.Context, machineStatus *omni.MachineStatus) {
	machineStatusSpec := machineStatus.TypedSpec().Value

	talosMachineStatus := machineStatusSpec.GetTalosMachineStatus()
	if talosMachineStatus == nil {
		return
	}

	if err := handler.handleMachineStatusEvent(ctx, talosMachineStatus.Status, machineStatus.Metadata().ID(),
		handler.Clock.Now(), false); err != nil {
		handler.logger.Error("error handling machine status machineStatus", zap.Error(err))
	}
}

func (handler *Handler) handleMachineDestroy(ctx context.Context, machine *omni.Machine) {
	id := machine.Metadata().ID()

	if err := handler.state.Destroy(ctx, resource.NewMetadata(resources.DefaultNamespace, omni.MachineStatusSnapshotType, id, resource.VersionUndefined)); err != nil {
		if !state.IsNotFoundError(err) {
			handler.logger.Error("error destroying machine status snapshot", zap.Error(err))
		}
	}

	handler.lock.Lock()
	delete(handler.machineToLastEventInfo, id)
	handler.lock.Unlock()
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
		return handler.handleMachineStatusEvent(ctx, event, machines.Get(0).Metadata().ID(), handler.Clock.Now(), true)
	default: // nothing, we ignore other events
	}

	return nil
}

func (handler *Handler) handleMachineStatusEvent(ctx context.Context, event *machineapi.MachineStatusEvent, machineID resource.ID, timestamp time.Time, isFromSiderolink bool) error {
	accept := handler.acceptEvent(machineID, timestamp, isFromSiderolink)

	handler.logger.Info("got machine status event",
		zap.String("machine", machineID),
		zap.String("stage", event.Stage.String()),
		zap.Any("status", event.Status),
		zap.Time("timestamp", timestamp),
		zap.Bool("from_siderolink", isFromSiderolink),
		zap.Bool("accept", accept))

	if !accept {
		return nil
	}

	snapshot := omni.NewMachineStatusSnapshot(resources.DefaultNamespace, machineID)

	if _, err := safe.StateUpdateWithConflicts(
		ctx,
		handler.state,
		snapshot.Metadata(),
		func(status *omni.MachineStatusSnapshot) error {
			status.TypedSpec().Value.MachineStatus = event

			return nil
		},
	); err != nil {
		if !state.IsNotFoundError(err) {
			return err
		}

		snapshot.TypedSpec().Value.MachineStatus = event

		return handler.state.Create(ctx, snapshot)
	}

	return nil
}

func (handler *Handler) acceptEvent(machineID resource.ID, timestamp time.Time, isFromSiderolink bool) bool {
	handler.lock.Lock()
	defer handler.lock.Unlock()

	currentEventInfo := eventInfo{
		timestamp:      timestamp,
		fromSideroLink: isFromSiderolink,
	}

	lastEventInfo, ok := handler.machineToLastEventInfo[machineID]

	accept := false

	switch {
	// The last event is zero, i.e., current event is the first event
	case !ok:
		accept = true
	// Both the source of the current event and the last one are from SideroLink
	case lastEventInfo.fromSideroLink && currentEventInfo.fromSideroLink:
		accept = true
	// if the current event is newer than the last one, we accept the current one
	case currentEventInfo.timestamp.After(lastEventInfo.timestamp):
		accept = true
	}

	if accept {
		handler.machineToLastEventInfo[machineID] = currentEventInfo
	}

	return accept
}
