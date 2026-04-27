// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
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

func TestMachineStatusMetricsController_UnsupportedTalosVersion(t *testing.T) {
	t.Parallel()

	require.True(t, omnictrl.UnsupportedTalosVersionNotificationEnabled, "this test assumes UnsupportedTalosVersionNotificationEnabled is true")

	for _, tt := range []struct {
		machineVersions         map[string]string
		name                    string
		expectApproachingCount  int
		expectEndOfSupportCount int
		expectApproaching       bool
		expectEndOfSupport      bool
	}{
		{
			name:            "all machines above threshold",
			machineVersions: map[string]string{"m1": "v1.10.0", "m2": "v1.11.0"},
		},
		{
			name:                   "machines approaching end of support",
			machineVersions:        map[string]string{"m1": "v1.8.0", "m2": "v1.9.0", "m3": "v1.11.0"},
			expectApproaching:      true,
			expectApproachingCount: 2,
		},
		{
			name:                    "machines past end of support",
			machineVersions:         map[string]string{"m1": "v1.7.0", "m2": "v1.6.0", "m3": "v1.11.0"},
			expectEndOfSupport:      true,
			expectEndOfSupportCount: 2,
		},
		{
			name:                    "mix of approaching and past end of support",
			machineVersions:         map[string]string{"m1": "v1.8.0", "m2": "v1.7.0", "m3": "v1.11.0"},
			expectApproaching:       true,
			expectApproachingCount:  1,
			expectEndOfSupport:      true,
			expectEndOfSupportCount: 1,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			t.Cleanup(cancel)

			testutils.WithRuntime(ctx, t, testutils.TestOptions{},
				func(_ context.Context, tc testutils.TestContext) {
					require.NoError(t, tc.Runtime.RegisterController(omnictrl.NewMachineStatusMetricsController(0)))
				},
				func(ctx context.Context, tc testutils.TestContext) {
					for id, version := range tt.machineVersions {
						ms := omni.NewMachineStatus(id)
						ms.TypedSpec().Value.TalosVersion = version

						require.NoError(t, tc.State.Create(ctx, ms))
					}

					if tt.expectApproaching {
						rtestutils.AssertResource(ctx, t, tc.State, omni.NotificationApproachingTalosVersionEndOfSupportID, func(res *omni.Notification, a *assert.Assertions) {
							a.Contains(res.TypedSpec().Value.Body, fmt.Sprintf("%d machine(s)", tt.expectApproachingCount))
							a.Equal(specs.NotificationSpec_WARNING, res.TypedSpec().Value.Type)
						})
					} else {
						// Notification should not exist. Sleep briefly since there is no state change to poll on.
						time.Sleep(500 * time.Millisecond)
						rtestutils.AssertNoResource[*omni.Notification](ctx, t, tc.State, omni.NotificationApproachingTalosVersionEndOfSupportID)
					}

					if tt.expectEndOfSupport {
						rtestutils.AssertResource(ctx, t, tc.State, omni.NotificationTalosVersionEndOfSupportID, func(res *omni.Notification, a *assert.Assertions) {
							a.Contains(res.TypedSpec().Value.Body, fmt.Sprintf("%d machine(s)", tt.expectEndOfSupportCount))
							a.Equal(specs.NotificationSpec_WARNING, res.TypedSpec().Value.Type)
						})
					} else {
						time.Sleep(500 * time.Millisecond)
						rtestutils.AssertNoResource[*omni.Notification](ctx, t, tc.State, omni.NotificationTalosVersionEndOfSupportID)
					}
				},
			)
		})
	}
}

func TestMachineStatusMetricsController_UnsupportedTalosVersionTeardown(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	t.Cleanup(cancel)

	testutils.WithRuntime(ctx, t, testutils.TestOptions{},
		func(_ context.Context, tc testutils.TestContext) {
			require.NoError(t, tc.Runtime.RegisterController(omnictrl.NewMachineStatusMetricsController(0)))
		},
		func(ctx context.Context, tc testutils.TestContext) {
			ms := omni.NewMachineStatus("m1")
			ms.TypedSpec().Value.TalosVersion = "v1.7.0"

			require.NoError(t, tc.State.Create(ctx, ms))

			rtestutils.AssertResource(ctx, t, tc.State, omni.NotificationTalosVersionEndOfSupportID, func(res *omni.Notification, a *assert.Assertions) {
				a.Equal(specs.NotificationSpec_WARNING, res.TypedSpec().Value.Type)
			})

			_, err := safe.StateUpdateWithConflicts(ctx, tc.State, ms.Metadata(), func(res *omni.MachineStatus) error {
				res.TypedSpec().Value.TalosVersion = "v1.11.0"

				return nil
			})
			require.NoError(t, err)

			rtestutils.AssertNoResource[*omni.Notification](ctx, t, tc.State, omni.NotificationTalosVersionEndOfSupportID)
			rtestutils.AssertNoResource[*omni.Notification](ctx, t, tc.State, omni.NotificationApproachingTalosVersionEndOfSupportID)
		},
	)
}
