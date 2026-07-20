// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package infraprovider_test

import (
	"context"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/infraprovider"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils/rmock"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils/rmock/options"
)

func TestCombinedStatusController(t *testing.T) {
	t.Parallel()

	const healthCheckInterval = time.Second

	addControllers := func(_ context.Context, testContext testutils.TestContext) {
		require.NoError(t, testContext.Runtime.RegisterQController(infraprovider.NewCombinedStatusController(healthCheckInterval)))
	}

	t.Run("disconnectedAfterHealthCheckInterval", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second*10)
		t.Cleanup(cancel)

		testutils.WithRuntime(
			ctx, t, testutils.TestOptions{}, addControllers,
			func(ctx context.Context, testContext testutils.TestContext) {
				providerID := "test-provider"

				rmock.Mock[*infra.Provider](
					ctx, t, testContext.State,
					options.WithID(providerID),
				)

				// Create a health status with a recent heartbeat - should be connected.
				rmock.Mock[*infra.ProviderHealthStatus](
					ctx, t, testContext.State,
					options.WithID(providerID),
					options.Modify(func(res *infra.ProviderHealthStatus) error {
						res.TypedSpec().Value.LastHeartbeatTimestamp = timestamppb.Now()

						return nil
					}),
				)

				rtestutils.AssertResources(
					ctx, t, testContext.State, []string{providerID},
					func(res *omni.InfraProviderCombinedStatus, assertions *assert.Assertions) {
						assertions.True(res.TypedSpec().Value.Health.Connected, "provider should be connected with a recent heartbeat")
					},
				)

				// Wait for the heartbeat to expire, then poke the provider health status to trigger reconciliation.
				time.Sleep(healthCheckInterval + time.Second)

				rmock.Mock[*infra.ProviderHealthStatus](
					ctx, t, testContext.State,
					options.WithID(providerID),
				)

				rtestutils.AssertResources(
					ctx, t, testContext.State, []string{providerID},
					func(res *omni.InfraProviderCombinedStatus, assertions *assert.Assertions) {
						assertions.False(res.TypedSpec().Value.Health.Connected, "provider should be disconnected after health check interval")
					},
				)
			},
		)
	})

	t.Run("propagatesProviderStatusFields", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second*10)
		t.Cleanup(cancel)

		testutils.WithRuntime(
			ctx, t, testutils.TestOptions{}, addControllers,
			func(ctx context.Context, testContext testutils.TestContext) {
				providerID := "test-provider"

				rmock.Mock[*infra.Provider](
					ctx, t, testContext.State,
					options.WithID(providerID),
				)

				rmock.Mock[*infra.ProviderStatus](
					ctx, t, testContext.State,
					options.WithID(providerID),
					options.Modify(func(res *infra.ProviderStatus) error {
						res.TypedSpec().Value.Name = "Test Provider"
						res.TypedSpec().Value.Description = "a test provider"
						res.TypedSpec().Value.Version = "v1.2.3"

						return nil
					}),
				)

				rtestutils.AssertResources(
					ctx, t, testContext.State, []string{providerID},
					func(res *omni.InfraProviderCombinedStatus, assertions *assert.Assertions) {
						assertions.Equal("Test Provider", res.TypedSpec().Value.Name)
						assertions.Equal("a test provider", res.TypedSpec().Value.Description)
						assertions.Equal("v1.2.3", res.TypedSpec().Value.Version, "provider version should be propagated to the combined status")
						assertions.True(res.TypedSpec().Value.Health.Initialized, "provider should be initialized once the provider status is present")
					},
				)
			},
		)
	})
}
