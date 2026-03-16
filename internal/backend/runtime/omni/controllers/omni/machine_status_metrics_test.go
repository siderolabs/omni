// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"context"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils"
)

func TestMachineStatusMetricsController_RegistrationLimit(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name               string
		machineIDs         []string
		maxRegistered      uint32
		expectCount        uint32
		expectLimit        uint32
		expectLimitReached bool
	}{
		{
			name:               "limit not reached",
			machineIDs:         []string{"m1", "m2"},
			maxRegistered:      5,
			expectCount:        2,
			expectLimit:        5,
			expectLimitReached: false,
		},
		{
			name:               "limit reached",
			machineIDs:         []string{"m1", "m2"},
			maxRegistered:      2,
			expectCount:        2,
			expectLimit:        2,
			expectLimitReached: true,
		},
		{
			name:               "limit exceeded",
			machineIDs:         []string{"m1", "m2", "m3"},
			maxRegistered:      1,
			expectCount:        3,
			expectLimit:        1,
			expectLimitReached: true,
		},
		{
			name:               "unlimited when zero",
			machineIDs:         []string{"m1", "m2"},
			maxRegistered:      0,
			expectCount:        2,
			expectLimit:        0,
			expectLimitReached: false,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			t.Cleanup(cancel)

			testutils.WithRuntime(ctx, t, testutils.TestOptions{},
				func(_ context.Context, tc testutils.TestContext) {
					require.NoError(t, tc.Runtime.RegisterController(omnictrl.NewMachineStatusMetricsController(tt.maxRegistered)))
				},
				func(ctx context.Context, tc testutils.TestContext) {
					for _, id := range tt.machineIDs {
						require.NoError(t, tc.State.Create(ctx, omni.NewMachineStatus(id)))
					}

					rtestutils.AssertResource(ctx, t, tc.State, omni.MachineStatusMetricsID, func(res *omni.MachineStatusMetrics, a *assert.Assertions) {
						a.EqualValues(tt.expectCount, res.TypedSpec().Value.RegisteredMachinesCount)
						a.EqualValues(tt.expectLimit, res.TypedSpec().Value.RegisteredMachinesLimit)
						a.Equal(tt.expectLimitReached, res.TypedSpec().Value.RegistrationLimitReached)
					})

					if tt.expectLimitReached {
						rtestutils.AssertResource(ctx, t, tc.State, omni.NotificationMachineRegistrationLimitID, func(res *omni.Notification, a *assert.Assertions) {
							a.Equal("Machine Registration Limit Reached", res.TypedSpec().Value.Title)
							a.Contains(res.TypedSpec().Value.Body, "machines registered")
							a.Equal(specs.NotificationSpec_WARNING, res.TypedSpec().Value.Type)
						})
					} else {
						// Notification should not exist when limit is not reached.
						// Sleep briefly since there is no state change to poll on.
						time.Sleep(500 * time.Millisecond)
						rtestutils.AssertNoResource[*omni.Notification](ctx, t, tc.State, omni.NotificationMachineRegistrationLimitID)
					}
				},
			)
		})
	}
}
