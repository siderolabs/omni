// Copyright (c) 2025 Sidero Labs, Inc.
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

// WatcherOptions contains additional options for the Watcher.
type WatcherOptions struct {
	// StartCh will be closed when the watcher starts running.
	StartCh chan<- struct{}

	// PostHandleNotifyCh is a channel to send a copy of raw state.Event notifications after they are processed.
	PostHandleNotifyCh chan<- state.Event
}

// Watcher watches the infra.MachineStatus resources and sends MachineStatusSnapshot resources with the power stage information to the snapshot channel.
type Watcher struct {
	state      state.State
	snapshotCh chan<- *omni.MachineStatusSnapshot
	logger     *zap.Logger
	options    WatcherOptions
}

// NewWatcher creates a new Watcher.
func NewWatcher(state state.State, snapshotCh chan<- *omni.MachineStatusSnapshot, logger *zap.Logger, options WatcherOptions) *Watcher {
	return &Watcher{
		state:      state,
		snapshotCh: snapshotCh,
		logger:     logger,
		options:    options,
	}
}

// Run runs the Watcher.
func (watcher *Watcher) Run(ctx context.Context) error {
	eventCh := make(chan state.Event)

	if err := watcher.state.WatchKind(ctx, infra.NewMachineStatus("").Metadata(), eventCh); err != nil {
		return err
	}

	if err := watcher.state.WatchKind(ctx, omni.NewClusterMachine(resources.DefaultNamespace, "").Metadata(), eventCh); err != nil {
		return fmt.Errorf("failed to watch cluster machines: %w", err)
	}

	if watcher.options.StartCh != nil {
		close(watcher.options.StartCh)
	}

	for {
		select {
		case <-ctx.Done():
			watcher.logger.Info("power status watcher stopped")

			return nil
		case event := <-eventCh:
			if err := watcher.handleEvent(ctx, event); err != nil {
				return err
			}
		}
	}
}

func (watcher *Watcher) handleEvent(ctx context.Context, event state.Event) error {
	defer watcher.notify(ctx, event)

	var (
		clusterMachineExists bool
		infraMachineStatus   *infra.MachineStatus
		err                  error
	)

	if event.Error != nil {
		return fmt.Errorf("power status watcher failed: %w", event.Error)
	}

	switch res := event.Resource.(type) {
	case *omni.ClusterMachine:
		if event.Type != state.Created {
			return nil
		}

		clusterMachineExists = true

		if infraMachineStatus, err = safe.StateGetByID[*infra.MachineStatus](ctx, watcher.state, res.Metadata().ID()); err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("failed to get infra machine status for cluster machine %s: %w", res.Metadata().ID(), err)
		}

		if infraMachineStatus == nil {
			return nil
		}
	case *infra.MachineStatus:
		if event.Type != state.Created && event.Type != state.Updated {
			return nil
		}

		infraMachineStatus = res

		_, err = watcher.state.Get(ctx, omni.NewClusterMachine(resources.DefaultNamespace, infraMachineStatus.Metadata().ID()).Metadata())
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("failed to get cluster machine for infra machine status %s: %w", infraMachineStatus.Metadata().ID(), err)
		}

		clusterMachineExists = err == nil
	}

	if infraMachineStatus.TypedSpec().Value.PowerState != specs.InfraMachineStatusSpec_POWER_STATE_OFF {
		return nil
	}

	snapshot := omni.NewMachineStatusSnapshot(resources.DefaultNamespace, infraMachineStatus.Metadata().ID())

	if clusterMachineExists { // the machine is assigned to a cluster, mark it as "powering on"
		snapshot.TypedSpec().Value.PowerStage = specs.MachineStatusSnapshotSpec_POWER_STAGE_POWERING_ON
	} else { // the machine is not assigned to a cluster, mark it as "powered off"
		snapshot.TypedSpec().Value.PowerStage = specs.MachineStatusSnapshotSpec_POWER_STAGE_POWERED_OFF
	}

	select {
	case <-ctx.Done():
		watcher.logger.Info("power status watcher stopped before sending a snapshot")
	case watcher.snapshotCh <- snapshot:
	}

	watcher.logger.Debug("sent power stage snapshot",
		zap.String("machine_id", infraMachineStatus.Metadata().ID()),
		zap.Stringer("power_stage", snapshot.TypedSpec().Value.PowerStage))

	return nil
}

func (watcher *Watcher) notify(ctx context.Context, event state.Event) {
	if watcher.options.PostHandleNotifyCh == nil {
		return
	}

	select {
	case <-ctx.Done():
		return
	case watcher.options.PostHandleNotifyCh <- event:
	}
}
