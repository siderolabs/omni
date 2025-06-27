// Copyright (c) 2025 Sidero Labs, Inc.
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
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
)

type ProviderJoinConfigSuite struct {
	OmniSuite
}

func (suite *ProviderJoinConfigSuite) TestReconcile() {
	suite.startRuntime()

	ctx, cancel := context.WithTimeout(suite.ctx, time.Second*5)

	defer cancel()

	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewProviderJoinConfigController()))

	apiConfig := siderolink.NewAPIConfig()
	apiConfig.TypedSpec().Value.EventsPort = 8091
	apiConfig.TypedSpec().Value.LogsPort = 8092
	apiConfig.TypedSpec().Value.MachineApiAdvertisedUrl = "http://127.0.0.1"

	params := siderolink.NewConnectionParams(resources.DefaultNamespace, siderolink.ConfigID)
	params.TypedSpec().Value.JoinToken = "jtoken"

	suite.Require().NoError(suite.state.Create(suite.ctx, params))
	suite.Require().NoError(suite.state.Create(suite.ctx, apiConfig))

	provider := infra.NewProvider("testprovider")

	suite.Require().NoError(suite.state.Create(ctx, provider))

	expectedConfig := "apiVersion: v1alpha1\n" +
		"kind: SideroLinkConfig\n" +
		"apiUrl: http://127.0.0.1?jointoken=v1%3AeyJleHRyYV9kYXRhIjp7Im9tbmkuc2lkZXJv" +
		"LmRldi9pbmZyYS1wcm92aWRlci1pZCI6InRlc3Rwcm92aWRlciJ9LCJzaWduYXR1cmUiOiJEQnRi" +
		"b0NDeURlQ1pBSXVkQnovTGhYSVNnMlVpZGViRzJad1hDa1ZCU3RvPSJ9\n" +
		"---\n" +
		"apiVersion: v1alpha1\n" +
		"kind: EventSinkConfig\n" +
		"endpoint: '[fdae:41e4:649b:9303::1]:8091'\n" +
		"---\n" +
		"apiVersion: v1alpha1\n" +
		"kind: KmsgLogConfig\n" +
		"name: omni-kmsg\n" +
		"url: tcp://[fdae:41e4:649b:9303::1]:8092\n"

	expectedKernelArgs := []string{
		"siderolink.api=http://127.0.0.1?jointoken=v1%3AeyJleHRyYV9kYXRhIjp7Im9tbmkuc2l" +
			"kZXJvLmRldi9pbmZyYS1wcm92aWRlci1pZCI6InRlc3Rwcm92aWRlciJ9LCJzaWduYXR1cmUiOiJ" +
			"EQnRib0NDeURlQ1pBSXVkQnovTGhYSVNnMlVpZGViRzJad1hDa1ZCU3RvPSJ9",
		"talos.events.sink=[fdae:41e4:649b:9303::1]:8091",
		"talos.logging.kernel=tcp://[fdae:41e4:649b:9303::1]:8092",
	}

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{provider.Metadata().ID()},
		func(res *siderolink.ProviderJoinConfig, assert *assert.Assertions) {
			assert.Equal(expectedConfig, res.TypedSpec().Value.Config.Config)
			assert.Equal(expectedKernelArgs, res.TypedSpec().Value.Config.KernelArgs)
		},
	)
}

func TestProviderJoinConfigSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(ProviderJoinConfigSuite))
}
