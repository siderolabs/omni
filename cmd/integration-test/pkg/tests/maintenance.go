// Copyright (c) 2025 Sidero Labs, Inc.
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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

const testPatch = `apiVersion: v1alpha1
kind: EventSinkConfig
endpoint: '[fdae:41e4:649b:9303::1]:8090'`

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
			buffer, bufferErr := r.TypedSpec().Value.GetUncompressedData()
			assertion.NoError(bufferErr)

			defer buffer.Free()

			data := string(buffer.Data())

			assertion.Contains(data, testPatch)
		})
	}
}
