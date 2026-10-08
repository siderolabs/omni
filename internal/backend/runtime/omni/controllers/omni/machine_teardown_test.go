// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/siderolabs/talos/pkg/machinery/api/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils"
)

func TestMachineTeardownWipe(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name           string
		connected      bool
		expectWipeDial bool
	}{
		{name: "connected", connected: true, expectWipeDial: true},
		{name: "disconnected", connected: false, expectWipeDial: false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			t.Cleanup(cancel)

			testutils.WithRuntime(
				ctx,
				t,
				testutils.TestOptions{},
				func(_ context.Context, testContext testutils.TestContext) {
					require.NoError(t, testContext.Runtime.RegisterQController(omnictrl.NewMachineTeardownController()))
				},
				func(ctx context.Context, testContext testutils.TestContext) {
					var disksCalled atomic.Bool

					machineService := testutils.NewMachineServices(t, testContext.State).Create(ctx, tt.name)
					machineService.SetDisksHandler(func(context.Context, *emptypb.Empty) (*storage.DisksResponse, error) {
						disksCalled.Store(true)

						return &storage.DisksResponse{}, nil
					})

					machineStatus := omni.NewMachineStatus(tt.name)
					machineStatus.TypedSpec().Value.Connected = tt.connected
					machineStatus.TypedSpec().Value.ManagementAddress = machineService.SocketConnectionString

					require.NoError(t, testContext.State.Create(ctx, machineStatus))

					rtestutils.AssertResource(ctx, t, testContext.State, machineStatus.Metadata().ID(), func(res *omni.MachineStatus, assertion *assert.Assertions) {
						assertion.True(res.Metadata().Finalizers().Has(omnictrl.MachineTeardownControllerName))
					})

					_, err := testContext.State.Teardown(ctx, machineStatus.Metadata())
					require.NoError(t, err)

					rtestutils.AssertResource(ctx, t, testContext.State, machineStatus.Metadata().ID(), func(res *omni.MachineStatus, assertion *assert.Assertions) {
						assertion.False(res.Metadata().Finalizers().Has(omnictrl.MachineTeardownControllerName))
					})

					assert.Equal(t, tt.expectWipeDial, disksCalled.Load())
				},
			)
		})
	}
}
