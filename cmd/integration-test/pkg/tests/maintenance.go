// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package tests

import (
	"context"
	"slices"
	"testing"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

const testPatch = `apiVersion: v1alpha1
kind: KmsgLogConfig
name: test-patch
url: tcp://[fdae:41e4:649b:9303::1]:12345`

// ApplyMaintenanceTestConfig applies a test configuration to a machine in maintenance mode.
func ApplyMaintenanceTestConfig(ctx context.Context, t *testing.T, omniState state.State, machineID resource.ID) {
	// apply config in maintenance mode
	machineStatus, err := safe.StateGetByID[*omni.MachineStatus](ctx, omniState, machineID)
	require.NoError(t, err)

	talosCli, err := talosClientMaintenance(ctx, machineStatus.TypedSpec().Value.GetManagementAddress())
	require.NoError(t, err)

	applyConfigurationReq := machine.ApplyConfigurationRequest{
		Data: []byte(testPatch),
	}

	_, err = talosCli.ApplyConfiguration(ctx, &applyConfigurationReq)
	require.NoError(t, err)

	t.Logf("applied maintenance config on machine %q", machineID)

	rtestutils.AssertResource[*omni.MachineStatus](ctx, t, omniState, machineID, func(r *omni.MachineStatus, assertion *assert.Assertions) {
		assertion.Contains(r.TypedSpec().Value.GetMaintenanceConfig().GetConfig(), testPatch)
	})
}

// AssertMaintenanceTestConfigIsPresent asserts that the test configuration is present on a machine in maintenance mode.
func AssertMaintenanceTestConfigIsPresent(ctx context.Context, omniState state.State, cluster resource.ID, machineIndex int) TestFunc {
	return func(t *testing.T) {
		machineStatusList, err := safe.StateListAll[*omni.MachineStatus](ctx, omniState, state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, cluster)))
		require.NoError(t, err)

		ids := make([]resource.ID, 0, machineStatusList.Len())

		machineStatusList.ForEach(func(status *omni.MachineStatus) { ids = append(ids, status.Metadata().ID()) })

		slices.Sort(ids)

		require.Less(t, machineIndex, len(ids), "machine index out of range")

		machineID := ids[machineIndex]

		rtestutils.AssertResource[*omni.RedactedClusterMachineConfig](ctx, t, omniState, machineID, func(r *omni.RedactedClusterMachineConfig, assertion *assert.Assertions) {
			assertion.Contains(r.TypedSpec().Value.Data, testPatch)
		})
	}
}
