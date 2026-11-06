// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build integration

package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	siderolinkclient "github.com/siderolabs/omni/client/pkg/siderolink"
)

func testTalosVersionFloorPrepare(options *TestOptions) TestFunc {
	return func(t *testing.T) {
		t.Log(`
Test the Talos version floor, the first part that runs without the disconnect flag
- wait for all machines, below and above the floor, to join and connect
- check that both kinds of machines are present
- check that the notification about the machines below the floor is shown`)

		testCtx := t.Context()
		st := options.omniClient.Omni().State()

		t.Run("LinkCountShouldMatchExpectedMachines", AssertNumberOfLinks(testCtx, st, expectedMachines))
		t.Run("LinksShouldBeConnected", AssertLinksConnected(testCtx, st))
		t.Run("MachinesBelowAndAboveFloorShouldBePresent", assertMachinesAroundTalosVersionFloor(testCtx, st))

		t.Run("BelowFloorNotificationShouldBeShown", func(t *testing.T) {
			ctx, cancel := context.WithTimeout(testCtx, time.Minute)
			t.Cleanup(cancel)

			rtestutils.AssertResources(ctx, t, st, []string{omni.NotificationTalosVersionBelowFloorID}, func(res *omni.Notification, assert *assert.Assertions) {
				assert.True(res.TypedSpec().Value.Undismissible)
			})
		})
	}
}

func testTalosVersionFloorVerify(options *TestOptions) TestFunc {
	return func(t *testing.T) {
		t.Log(`
Test the Talos version floor, the second part that runs with the disconnect flag
- check that the machines below the floor are disconnected
- check that the machines at or above the floor stay connected
- check that the notification about the machines below the floor can be dismissed`)

		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Minute)
		t.Cleanup(cancel)

		st := options.omniClient.Omni().State()

		t.Run("MachinesBelowAndAboveFloorShouldBePresent", assertMachinesAroundTalosVersionFloor(ctx, st))

		t.Run("OnlyMachinesBelowFloorShouldBeDisconnected", func(t *testing.T) {
			rtestutils.AssertAll(ctx, t, st, func(machineStatus *omni.MachineStatus, assert *assert.Assertions) {
				belowFloor := siderolinkclient.IsBelowTalosVersionFloor(machineStatus.TypedSpec().Value.TalosVersion)

				link, err := safe.StateGetByID[*siderolink.Link](ctx, st, machineStatus.Metadata().ID())
				if !assert.NoError(err) {
					return
				}

				assert.Equal(!belowFloor, link.TypedSpec().Value.Connected, resourceDetails(machineStatus))
				assert.Equal(!belowFloor, machineStatus.TypedSpec().Value.Connected, resourceDetails(machineStatus))
			})
		})

		t.Run("BelowFloorNotificationShouldBeDismissable", func(t *testing.T) {
			rtestutils.AssertResources(ctx, t, st, []string{omni.NotificationTalosVersionBelowFloorDisconnectedID}, func(res *omni.Notification, assert *assert.Assertions) {
				assert.False(res.TypedSpec().Value.Undismissible)
			})

			rtestutils.AssertNoResource[*omni.Notification](ctx, t, st, omni.NotificationTalosVersionBelowFloorID)
		})
	}
}

func assertMachinesAroundTalosVersionFloor(testCtx context.Context, st state.State) TestFunc {
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(testCtx, 5*time.Minute)
		t.Cleanup(cancel)

		rtestutils.AssertAll(ctx, t, st, func(machineStatus *omni.MachineStatus, assert *assert.Assertions) {
			assert.NotEmpty(machineStatus.TypedSpec().Value.TalosVersion, resourceDetails(machineStatus))
		})

		machineStatuses, err := safe.StateListAll[*omni.MachineStatus](ctx, st)
		require.NoError(t, err)

		var below, above int

		for machineStatus := range machineStatuses.All() {
			if siderolinkclient.IsBelowTalosVersionFloor(machineStatus.TypedSpec().Value.TalosVersion) {
				below++
			} else {
				above++
			}
		}

		require.Positive(t, below, "expected machines below the Talos version floor %s", constants.TalosVersionFloor)
		require.Positive(t, above, "expected machines at or above the Talos version floor %s", constants.TalosVersionFloor)
	}
}
