// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package snapshot_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/siderolabs/talos/pkg/machinery/api/common"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/task/snapshot"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils"
)

const testBootID = "11111111-2222-3333-4444-555555555555"

func TestCollectTaskBootID(t *testing.T) {
	t.Parallel()

	t.Run("boot ID is reported", func(t *testing.T) {
		t.Parallel()

		ctx, spec, _ := setupCollectTask(t, "v1.14.5")

		assert.Equal(t, testBootID, firstSnapshot(ctx, t, spec).TypedSpec().Value.BootId)
	})

	t.Run("a failed read restarts the task", func(t *testing.T) {
		t.Parallel()

		ctx, spec, machineService := setupCollectTask(t, "v1.14.5")

		var calls atomic.Int32

		machineService.SetReadHandler(bootIDReadHandler(func() error {
			if calls.Add(1) == 1 {
				return status.Error(codes.Unavailable, "connection is closing")
			}

			return nil
		}))

		// the task does not report an empty boot ID, it returns the error so that the task runner restarts it
		err := spec.RunTask(ctx, zaptest.NewLogger(t), make(chan *omni.MachineStatusSnapshot))
		require.ErrorContains(t, err, "failed to read boot id")

		assert.Equal(t, testBootID, firstSnapshot(ctx, t, spec).TypedSpec().Value.BootId)
	})

	t.Run("a failed version check restarts the task", func(t *testing.T) {
		t.Parallel()

		ctx, spec, machineService := setupCollectTask(t, "v1.14.5")

		var calls atomic.Int32

		machineService.SetVersionHandler(func(context.Context, *emptypb.Empty) (*machine.VersionResponse, error) {
			if calls.Add(1) == 1 {
				return nil, status.Error(codes.Unavailable, "connection is closing")
			}

			return versionResponse("v1.14.5"), nil
		})

		// a failed check does not mean that the machine does not serve the boot ID, the task restarts instead
		err := spec.RunTask(ctx, zaptest.NewLogger(t), make(chan *omni.MachineStatusSnapshot))
		require.ErrorContains(t, err, "failed to read talos version")

		assert.Equal(t, testBootID, firstSnapshot(ctx, t, spec).TypedSpec().Value.BootId)
	})

	t.Run("boot ID is skipped below Talos 1.13", func(t *testing.T) {
		t.Parallel()

		ctx, spec, machineService := setupCollectTask(t, "v1.12.12")

		machineService.SetReadHandler(bootIDReadHandler(func() error {
			return status.Error(codes.Unimplemented, "not served in maintenance mode")
		}))

		assert.Empty(t, firstSnapshot(ctx, t, spec).TypedSpec().Value.BootId)
	})
}

// setupCollectTask starts a machine running the given Talos version, which serves the boot ID.
func setupCollectTask(t *testing.T, talosVersion string) (context.Context, snapshot.CollectTaskSpec, *testutils.MachineServiceMock) {
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	t.Cleanup(cancel)

	machineService := testutils.NewMachineServiceMock(ctx, t, "machine", state.WrapCore(namespaced.NewState(inmem.Build)))

	machineService.SetVersionHandler(func(context.Context, *emptypb.Empty) (*machine.VersionResponse, error) {
		return versionResponse(talosVersion), nil
	})

	machineService.SetReadHandler(bootIDReadHandler(func() error { return nil }))

	return ctx, snapshot.CollectTaskSpec{
		Endpoint:  machineService.SocketConnectionString,
		MachineID: "machine",
	}, machineService
}

// bootIDReadHandler serves the boot ID, unless readErr returns an error.
func bootIDReadHandler(readErr func() error) func(*machine.ReadRequest, grpc.ServerStreamingServer[common.Data]) error {
	return func(req *machine.ReadRequest, srv grpc.ServerStreamingServer[common.Data]) error {
		if req.GetPath() != "/proc/sys/kernel/random/boot_id" {
			return status.Error(codes.NotFound, "not found")
		}

		if err := readErr(); err != nil {
			return err
		}

		return srv.Send(&common.Data{Bytes: []byte(testBootID + "\n")})
	}
}

func versionResponse(tag string) *machine.VersionResponse {
	return &machine.VersionResponse{
		Messages: []*machine.Version{
			{
				Version: &machine.VersionInfo{Tag: tag},
			},
		},
	}
}

// firstSnapshot runs the task until it sends its first snapshot, which it does for the initial machine status.
func firstSnapshot(ctx context.Context, t *testing.T, spec snapshot.CollectTaskSpec) *omni.MachineStatusSnapshot {
	ctx, cancel := context.WithCancel(ctx)

	notifyCh := make(chan *omni.MachineStatusSnapshot)
	errCh := make(chan error, 1)

	go func() {
		errCh <- spec.RunTask(ctx, zaptest.NewLogger(t), notifyCh)
	}()

	var snap *omni.MachineStatusSnapshot

	select {
	case snap = <-notifyCh:
	case err := <-errCh:
		cancel()
		t.Fatalf("the task returned before sending a snapshot: %v", err)
	case <-ctx.Done():
		cancel()
		t.Fatal("timed out waiting for a snapshot")
	}

	cancel()

	// the watch can report the cancellation as its own error before the task sees the context done
	if err := <-errCh; err != nil {
		require.ErrorIs(t, err, context.Canceled)
	}

	return snap
}
