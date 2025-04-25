// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"context"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
)

type InfraProviderCleanupSuite struct {
	OmniSuite
}

func (suite *InfraProviderCleanupSuite) TestCleanup() {
	require := suite.Require()

	suite.ctx, suite.ctxCancel = context.WithTimeout(suite.ctx, time.Second*10)

	suite.startRuntime()

	controller := omnictrl.NewInfraProviderCleanupController()

	require.NoError(suite.runtime.RegisterController(controller))

	providerID := "infra-provider"

	provider := infra.NewProvider(providerID)
	providerStatus := infra.NewProviderStatus(providerID)
	providerHealthStatus := infra.NewProviderHealthStatus(providerID)

	provider.Metadata().Finalizers().Add(controller.Name())

	require.NoError(suite.state.Create(suite.ctx, provider))
	require.NoError(suite.state.Create(suite.ctx, providerStatus))
	require.NoError(suite.state.Create(suite.ctx, providerHealthStatus))

	_, err := suite.state.Teardown(suite.ctx, provider.Metadata())

	require.NoError(err)

	assertNoResource(&suite.OmniSuite, providerStatus)
	assertNoResource(&suite.OmniSuite, providerHealthStatus)

	assertResource[*omni.Machine](&suite.OmniSuite, provider.Metadata(), func(r *omni.Machine, assertion *assert.Assertions) {
		assertion.Empty(r.Metadata().Finalizers())
	})

	require.NoError(suite.state.Destroy(suite.ctx, provider.Metadata()))
}
