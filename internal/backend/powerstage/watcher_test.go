// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package powerstage_test

import (
	"context"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/powerstage"
)

func TestWatcher(t *testing.T) {
	st := state.WrapCore(namespaced.NewState(inmem.Build))
	snapshotCh := make(chan *omni.MachineStatusSnapshot)
	logger := zaptest.NewLogger(t)

	notifyCh := make(chan state.Event)
	startCh := make(chan struct{})
	watcherOpts := powerstage.WatcherOptions{
		StartCh:            startCh,
		PostHandleNotifyCh: notifyCh,
	}

	watcher := powerstage.NewWatcher(st, snapshotCh, logger, watcherOpts)

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	var eg errgroup.Group

	eg.Go(func() error {
		return watcher.Run(ctx)
	})

	expectStart(ctx, t, startCh)

	status := infra.NewMachineStatus("test")
	status.TypedSpec().Value.PowerState = specs.InfraMachineStatusSpec_POWER_STATE_OFF

	require.NoError(t, st.Create(ctx, status))

	expectSnapshot(ctx, t, snapshotCh, status.Metadata().ID(), specs.MachineStatusSnapshotSpec_POWER_STAGE_POWERED_OFF)
	expectNotification(ctx, t, notifyCh)

	clusterMachine := omni.NewClusterMachine(status.Metadata().ID())
	require.NoError(t, st.Create(ctx, clusterMachine))

	expectSnapshot(ctx, t, snapshotCh, clusterMachine.Metadata().ID(), specs.MachineStatusSnapshotSpec_POWER_STAGE_POWERING_ON)
	expectNotification(ctx, t, notifyCh)

	_, err := safe.StateUpdateWithConflicts(ctx, st, clusterMachine.Metadata(), func(res *omni.ClusterMachine) error {
		res.TypedSpec().Value.KubernetesVersion = "1.25.0"

		return nil
	})
	require.NoError(t, err)

	expectNotification(ctx, t, notifyCh)

	require.NoError(t, st.Destroy(ctx, status.Metadata()))

	expectNotification(ctx, t, notifyCh)

	require.NoError(t, st.Destroy(ctx, clusterMachine.Metadata()))

	expectNotification(ctx, t, notifyCh)

	cancel()
	require.NoError(t, eg.Wait())
}

func expectStart(ctx context.Context, t *testing.T, startCh chan struct{}) {
	select {
	case <-ctx.Done():
		require.Fail(t, "context timed out before watcher started")
	case <-startCh:
	}
}

func expectSnapshot(ctx context.Context, t *testing.T, ch <-chan *omni.MachineStatusSnapshot, id string, powerStage specs.MachineStatusSnapshotSpec_PowerStage) {
	select {
	case <-ctx.Done():
		require.Fail(t, "context timed out before snapshot received")
	case snapshot := <-ch:
		assert.Equal(t, id, snapshot.Metadata().ID())
		assert.Equal(t, powerStage, snapshot.TypedSpec().Value.PowerStage)
	}
}

func expectNotification(ctx context.Context, t *testing.T, notifyCh chan state.Event) {
	select {
	case <-ctx.Done():
		require.Fail(t, "context timed out before notification received")
	case <-notifyCh:
	}
}
