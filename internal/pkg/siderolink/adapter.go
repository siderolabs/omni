// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package siderolink implements siderolink manager.
package siderolink

import (
	"context"
	"fmt"
	"net/netip"

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

// NewEventsAdapter creates new Events.
func NewEventsAdapter(state state.State, logger *zap.Logger) *Adapter {
	return &Adapter{
		state:  state,
		logger: logger,
	}
}

// Adapter maintains machine status state based on the incoming events.
type Adapter struct {
	logger *zap.Logger
	state  state.State
}

// HandleEvent is called on each event coming from the Talos nodes.
func (e *Adapter) HandleEvent(ctx context.Context, event events.Event) error {
	ctx = actor.MarkContextAsInternalActor(ctx)

	ipPort, err := netip.ParseAddrPort(event.Node)
	if err != nil {
		return err
	}

	ip := ipPort.Addr().String()

	machines, err := safe.StateListAll[*omni.Machine](
		ctx,
		e.state,
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
		return e.handleMachineStatusEvent(ctx, event, machines.Get(0))
	default: // nothing, we ignore other events
	}

	return nil
}

// CleanupMachineStatuses cleans up machine status snapshot when machine is destroyed.
func (e *Adapter) CleanupMachineStatuses(ctx context.Context, logger *zap.Logger) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	watchCh := make(chan state.Event)

	if err := e.state.WatchKind(ctx,
		resource.NewMetadata(resources.DefaultNamespace, omni.MachineType, "", resource.VersionUndefined),
		watchCh,
	); err != nil {
		return fmt.Errorf("error watching machines: %w", err)
	}

	for {
		var event state.Event

		select {
		case <-ctx.Done():
			return nil
		case event = <-watchCh:
		}

		if event.Type != state.Destroyed {
			continue
		}

		id := event.Resource.Metadata().ID()

		if err := e.state.Destroy(ctx, resource.NewMetadata(resources.DefaultNamespace, omni.MachineStatusSnapshotType, id, resource.VersionUndefined)); err != nil {
			if !state.IsNotFoundError(err) {
				logger.Error("error destroying machine status snapshot", zap.Error(err))
			}
		}
	}
}

func (e *Adapter) handleMachineStatusEvent(ctx context.Context, event *machineapi.MachineStatusEvent, machine *omni.Machine) error {
	e.logger.Info("got machine status event",
		zap.String("machine", machine.Metadata().ID()),
		zap.String("stage", event.Stage.String()),
		zap.Any("status", event.Status),
	)

	snapshot := omni.NewMachineStatusSnapshot(resources.DefaultNamespace, machine.Metadata().ID())

	if _, err := safe.StateUpdateWithConflicts(
		ctx,
		e.state,
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

		return e.state.Create(ctx, snapshot)
	}

	return nil
}
