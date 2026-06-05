// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package machineconfig_test

import (
	"context"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/machineconfig"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils/rmock"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils/rmock/options"
)

// TestMachineDiscoveryServiceConfig verifies that the config status controller publishes the
// discovery service endpoint carried on the generated config into MachineDiscoveryServiceConfig once
// the config is applied, that it tracks changes to that endpoint, and that it is cleaned up on
// teardown. Downstream teardown logic uses this endpoint from the config Omni generated instead of
// reading it back from the node.
func TestMachineDiscoveryServiceConfig(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), time.Second*30)
	t.Cleanup(cancel)

	testutils.WithRuntime(
		ctx, t, testutils.TestOptions{},
		func(_ context.Context, tc testutils.TestContext) {
			require.NoError(t, tc.Runtime.RegisterQController(machineconfig.NewClusterMachineConfigStatusController(imageFactoryHost, "ghcr.io/siderolabs/installer")))
		},
		func(ctx context.Context, tc testutils.TestContext) {
			machineServices := testutils.NewMachineServices(t, tc.State)

			_, machines := createCluster(ctx, t, tc.State, machineServices, "discovery-config", 1, 0)
			require.Len(t, machines, 1)

			id := machines[0].Metadata().ID()

			// The resource is produced once the config is applied.
			rtestutils.AssertResource(ctx, t, tc.State, id, func(res *omni.MachineDiscoveryServiceConfig, a *assert.Assertions) {
				a.NotNil(res.TypedSpec().Value)
			})

			// The published endpoint follows the value carried on the generated config, not anything
			// read back from the node.
			const customEndpoint = "https://custom.discovery.example.com:8443"

			rmock.Mock[*omni.ClusterMachineConfig](
				ctx, t, tc.State,
				options.WithID(id),
				options.Modify(func(res *omni.ClusterMachineConfig) error {
					res.TypedSpec().Value.DiscoveryServiceEndpoint = customEndpoint

					return nil
				}),
			)

			rtestutils.AssertResource(ctx, t, tc.State, id, func(res *omni.MachineDiscoveryServiceConfig, a *assert.Assertions) {
				a.Equal(customEndpoint, res.TypedSpec().Value.DiscoveryServiceEndpoint)
			})

			// On teardown the resource is cleaned up. Let the machine reach maintenance on reset so the
			// config teardown can complete.
			machineServices.ForEach(func(m *testutils.MachineServiceMock) {
				m.OnReset = func(ctx context.Context, _ *machine.ResetRequest, st state.State, machineID string) (*machine.ResetResponse, error) {
					if err := safe.StateModify(ctx, st, omni.NewMachineStatusSnapshot(machineID), func(res *omni.MachineStatusSnapshot) error {
						res.TypedSpec().Value.MachineStatus.Stage = machine.MachineStatusEvent_MAINTENANCE

						return nil
					}); err != nil {
						return nil, err
					}

					return &machine.ResetResponse{}, nil
				}
			})

			rmock.Destroy[*omni.ClusterMachineConfig](ctx, t, tc.State, []string{id})

			rtestutils.AssertNoResource[*omni.MachineDiscoveryServiceConfig](ctx, t, tc.State, id)
		},
	)
}
