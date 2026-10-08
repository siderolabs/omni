// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	stdruntime "runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/siderolabs/talos/pkg/machinery/api/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils"
)

func TestMachineTeardownSkipDisconnected(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	t.Cleanup(cancel)

	managementAddress, disks := startRecordingDisksServer(t)

	testutils.WithRuntime(
		ctx,
		t,
		testutils.TestOptions{},
		func(_ context.Context, testContext testutils.TestContext) {
			require.NoError(t, testContext.Runtime.RegisterQController(omnictrl.NewMachineTeardownController()))
		},
		func(ctx context.Context, testContext testutils.TestContext) {
			machineStatus := omni.NewMachineStatus("disconnected")
			machineStatus.TypedSpec().Value.Connected = false
			machineStatus.TypedSpec().Value.ManagementAddress = managementAddress

			require.NoError(t, testContext.State.Create(ctx, machineStatus))

			rtestutils.AssertResource(ctx, t, testContext.State, machineStatus.Metadata().ID(), func(res *omni.MachineStatus, assertion *assert.Assertions) {
				assertion.True(res.Metadata().Finalizers().Has(omnictrl.MachineTeardownControllerName))
			})

			_, err := testContext.State.Teardown(ctx, machineStatus.Metadata())
			require.NoError(t, err)

			// Stay under the 10s wipe timeout so a dial that hangs fails this assertion.
			assertCtx, assertCancel := context.WithTimeout(ctx, 2*time.Second)
			defer assertCancel()

			rtestutils.AssertResource(assertCtx, t, testContext.State, machineStatus.Metadata().ID(), func(res *omni.MachineStatus, assertion *assert.Assertions) {
				assertion.False(res.Metadata().Finalizers().Has(omnictrl.MachineTeardownControllerName))
			})

			assert.False(t, disks.called.Load())
		},
	)
}

// recordingDisksServer records whether the wipe path asked Talos for disks.
type recordingDisksServer struct {
	storage.UnimplementedStorageServiceServer

	called atomic.Bool
}

func (s *recordingDisksServer) Disks(context.Context, *emptypb.Empty) (*storage.DisksResponse, error) {
	s.called.Store(true)

	return &storage.DisksResponse{}, nil
}

// startRecordingDisksServer serves a mock Talos storage API on a unix socket.
func startRecordingDisksServer(t *testing.T) (string, *recordingDisksServer) {
	t.Helper()

	socketPath := filepath.Join(t.TempDir(), "socket")

	if stdruntime.GOOS == "darwin" {
		// t.TempDir() exceeds the macOS unix socket path limit (104 bytes).
		temp, err := os.MkdirTemp("", "test-*****") //nolint:usetesting
		require.NoError(t, err)

		t.Cleanup(func() {
			require.NoError(t, os.RemoveAll(temp))
		})

		socketPath = filepath.Join(temp, "socket")
	}

	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "unix", socketPath)
	require.NoError(t, err)

	srv := grpc.NewServer()
	t.Cleanup(srv.Stop)

	disks := &recordingDisksServer{}

	storage.RegisterStorageServiceServer(srv, disks)

	go func() {
		for {
			serveErr := srv.Serve(listener)
			if serveErr == nil || errors.Is(serveErr, grpc.ErrServerStopped) {
				return
			}
		}
	}()

	return unixSocket + socketPath, disks
}
