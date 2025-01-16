// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package powerstage provides a power stage watcher that produces MachineStatusSnapshot events containing power stage information.
package powerstage

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// Watcher watches the infra.MachineStatus resources and sends MachineStatusSnapshot resources with the power stage information to the snapshot channel.
type Watcher struct {
	state      state.State
	snapshotCh chan<- *omni.MachineStatusSnapshot
	logger     *zap.Logger
}

// NewWatcher creates a new Watcher.
func NewWatcher(state state.State, snapshotCh chan<- *omni.MachineStatusSnapshot, logger *zap.Logger) *Watcher {
	return &Watcher{
		state:      state,
		snapshotCh: snapshotCh,
		logger:     logger,
	}
}

// Run runs the Watcher.
func (watcher *Watcher) Run(ctx context.Context) error {
	eventCh := make(chan safe.WrappedStateEvent[*infra.MachineStatus])

	if err := safe.StateWatchKind[*infra.MachineStatus](ctx, watcher.state, infra.NewMachineStatus("").Metadata(), eventCh); err != nil {
		return err
	}

	for {
		var event safe.WrappedStateEvent[*infra.MachineStatus]

		select {
		case <-ctx.Done():
			watcher.logger.Info("power status watcher stopped")

			return nil
		case event = <-eventCh:
		}

		switch event.Type() { //nolint:exhaustive
		case state.Created, state.Updated:
		default: // ignore other events
			continue
		}

		if err := event.Error(); err != nil {
			return fmt.Errorf("failed to watch machine status: %w", err)
		}

		resource, err := event.Resource()
		if err != nil {
			return fmt.Errorf("failed to read resource from the event: %w", err)
		}

		if resource.TypedSpec().Value.PowerState == specs.InfraMachineStatusSpec_POWER_STATE_OFF {
			snapshot := omni.NewMachineStatusSnapshot(resources.DefaultNamespace, resource.Metadata().ID())

			// find out if it is assigned to a cluster, and if so, mark it as "powering on"
			if _, err = watcher.state.Get(ctx, omni.NewClusterMachine(resources.DefaultNamespace, resource.Metadata().ID()).Metadata()); err != nil {
				if !state.IsNotFoundError(err) {
					return err
				}

				snapshot.TypedSpec().Value.PowerStage = specs.MachineStatusSnapshotSpec_POWER_STAGE_POWERED_OFF
			} else {
				snapshot.TypedSpec().Value.PowerStage = specs.MachineStatusSnapshotSpec_POWER_STAGE_POWERING_ON
			}

			select {
			case <-ctx.Done():
				watcher.logger.Info("power status watcher stopped before sending a snapshot")
			case watcher.snapshotCh <- snapshot:
			}

			watcher.logger.Debug("sent power stage snapshot",
				zap.String("machine_id", resource.Metadata().ID()),
				zap.Stringer("power_stage", snapshot.TypedSpec().Value.PowerStage))
		}
	}
}
