// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build integration

package integration_test

import (
	"context"
	"fmt"
	"net"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	siderolinkres "github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	siderolinkclient "github.com/siderolabs/omni/client/pkg/siderolink"
)

// AssertMaintenanceTestConfigIsPresent asserts that the test configuration is present on a machine in maintenance mode.
func AssertMaintenanceTestConfigIsPresent(ctx context.Context, omniState state.State, cluster resource.ID, machineIndex int) TestFunc {
	return func(t *testing.T) {
		timeoutCtx, cancel := context.WithTimeout(ctx, time.Minute*5)
		defer cancel()

		machineStatusList, err := safe.StateListAll[*omni.MachineStatus](timeoutCtx, omniState, state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, cluster)))
		require.NoError(t, err)

		ids := make([]resource.ID, 0, machineStatusList.Len())

		machineStatusList.ForEach(func(status *omni.MachineStatus) { ids = append(ids, status.Metadata().ID()) })

		slices.Sort(ids)

		require.Less(t, machineIndex, len(ids), "machine index out of range")

		machineID := ids[machineIndex]

		// Build the expected event sink endpoint the same way the machine config generation builds it.
		apiConfig, err := safe.StateGetByID[*siderolinkres.APIConfig](timeoutCtx, omniState, siderolinkres.ConfigID)
		require.NoError(t, err)

		sinkEndpoint := net.JoinHostPort(siderolinkclient.GetListenHost(), strconv.Itoa(int(apiConfig.TypedSpec().Value.EventsPort)))

		testPatch := fmt.Sprintf("apiVersion: v1alpha1\nkind: EventSinkConfig\nendpoint: '%s'", sinkEndpoint)

		rtestutils.AssertResource[*omni.RedactedClusterMachineConfig](timeoutCtx, t, omniState, machineID, func(r *omni.RedactedClusterMachineConfig, assertion *assert.Assertions) {
			buffer, bufferErr := r.TypedSpec().Value.GetUncompressedData()
			assertion.NoError(bufferErr)

			defer buffer.Free()

			data := string(buffer.Data())

			assertion.Contains(data, testPatch)
		})
	}
}
