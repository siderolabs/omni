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

func newNonImageFactoryDeprecationConfig(enabled bool) omnictrl.NonImageFactoryDeprecationConfig {
	return omnictrl.NonImageFactoryDeprecationConfig{
		Enabled: enabled,
		Title:   "Non-ImageFactory Machines Detected",
		Body:    "%d machine(s) were provisioned without ImageFactory.",
	}
}

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
					require.NoError(t, tc.Runtime.RegisterController(omnictrl.NewMachineStatusMetricsController(tt.maxRegistered, omnictrl.NonImageFactoryDeprecationConfig{})))
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

func TestMachineStatusMetricsController_NonImageFactoryDeprecation(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name                string
		invalidSchematicIDs []string
		validSchematicIDs   []string
		enabled             bool
		expectNotification  bool
		expectCount         int
	}{
		{
			name:                "disabled, invalid machines present",
			invalidSchematicIDs: []string{"m1"},
			enabled:             false,
			expectNotification:  false,
		},
		{
			name:               "enabled, no invalid machines",
			validSchematicIDs:  []string{"m1", "m2"},
			enabled:            true,
			expectNotification: false,
		},
		{
			name:                "enabled, some invalid machines",
			invalidSchematicIDs: []string{"m1", "m2"},
			validSchematicIDs:   []string{"m3"},
			enabled:             true,
			expectNotification:  true,
			expectCount:         2,
		},
		{
			name:                "enabled, all invalid machines",
			invalidSchematicIDs: []string{"m1", "m2", "m3"},
			enabled:             true,
			expectNotification:  true,
			expectCount:         3,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			t.Cleanup(cancel)

			testutils.WithRuntime(ctx, t, testutils.TestOptions{},
				func(_ context.Context, tc testutils.TestContext) {
					require.NoError(t, tc.Runtime.RegisterController(
						omnictrl.NewMachineStatusMetricsController(0, newNonImageFactoryDeprecationConfig(tt.enabled)),
					))
				},
				func(ctx context.Context, tc testutils.TestContext) {
					for _, id := range tt.invalidSchematicIDs {
						ms := omni.NewMachineStatus(id)
						ms.TypedSpec().Value.Schematic = &specs.MachineStatusSpec_Schematic{
							Invalid: true,
						}

						require.NoError(t, tc.State.Create(ctx, ms))
					}

					for _, id := range tt.validSchematicIDs {
						ms := omni.NewMachineStatus(id)
						ms.TypedSpec().Value.Schematic = &specs.MachineStatusSpec_Schematic{
							Id:     "valid-id",
							FullId: "valid-full-id",
						}

						require.NoError(t, tc.State.Create(ctx, ms))
					}

					if tt.expectNotification {
						rtestutils.AssertResource(ctx, t, tc.State, omni.NotificationNonImageFactoryMachinesID, func(res *omni.Notification, a *assert.Assertions) {
							a.Equal("Non-ImageFactory Machines Detected", res.TypedSpec().Value.Title)
							a.Contains(res.TypedSpec().Value.Body, fmt.Sprintf("%d machine(s)", tt.expectCount))
							a.Equal(specs.NotificationSpec_WARNING, res.TypedSpec().Value.Type)
						})
					} else {
						// Notification should not exist. Sleep briefly since there is no state change to poll on.
						time.Sleep(500 * time.Millisecond)
						rtestutils.AssertNoResource[*omni.Notification](ctx, t, tc.State, omni.NotificationNonImageFactoryMachinesID)
					}
				},
			)
		})
	}
}

func TestMachineStatusMetricsController_NonImageFactoryDeprecationTeardown(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	t.Cleanup(cancel)

	testutils.WithRuntime(ctx, t, testutils.TestOptions{},
		func(_ context.Context, tc testutils.TestContext) {
			require.NoError(t, tc.Runtime.RegisterController(
				omnictrl.NewMachineStatusMetricsController(0, newNonImageFactoryDeprecationConfig(true)),
			))
		},
		func(ctx context.Context, tc testutils.TestContext) {
			// Create a machine with invalid schematic.
			ms := omni.NewMachineStatus("m1")
			ms.TypedSpec().Value.Schematic = &specs.MachineStatusSpec_Schematic{Invalid: true}

			require.NoError(t, tc.State.Create(ctx, ms))

			// Wait for the notification to appear.
			rtestutils.AssertResource(ctx, t, tc.State, omni.NotificationNonImageFactoryMachinesID, func(res *omni.Notification, a *assert.Assertions) {
				a.Equal(specs.NotificationSpec_WARNING, res.TypedSpec().Value.Type)
			})

			// Fix the machine schematic (no longer invalid).
			_, err := safe.StateUpdateWithConflicts(ctx, tc.State, ms.Metadata(), func(res *omni.MachineStatus) error {
				res.TypedSpec().Value.Schematic = &specs.MachineStatusSpec_Schematic{
					Id:     "valid-id",
					FullId: "valid-full-id",
				}

				return nil
			})
			require.NoError(t, err)

			// Notification should be torn down.
			rtestutils.AssertNoResource[*omni.Notification](ctx, t, tc.State, omni.NotificationNonImageFactoryMachinesID)
		},
	)
}
