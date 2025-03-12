// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"context"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/omni/client/pkg/jointoken"
	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
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

	defaultToken := siderolink.NewDefaultJoinToken()
	defaultToken.TypedSpec().Value.TokenId = "jtoken"

	suite.Require().NoError(suite.state.Create(suite.ctx, defaultToken))
	suite.Require().NoError(suite.state.Create(suite.ctx, apiConfig))

	provider := infra.NewProvider("testprovider")

	suite.Require().NoError(suite.state.Create(ctx, provider))

	var token string

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{provider.Metadata().ID()},
		func(res *siderolink.ProviderJoinConfig, assert *assert.Assertions) {
			token = res.TypedSpec().Value.JoinToken

			assert.NotEmpty(token)
		},
	)

	jt, err := jointoken.NewWithExtraData(token, map[string]string{
		omni.LabelInfraProviderID: provider.Metadata().ID(),
	})

	suite.Require().NoError(err)

	expectedToken, err := jt.Encode()

	suite.Require().NoError(err)

	expectedToken = url.QueryEscape(expectedToken)

	expectedConfig := fmt.Sprintf(`apiVersion: v1alpha1
kind: SideroLinkConfig
apiUrl: http://127.0.0.1?jointoken=%s
---
apiVersion: v1alpha1
kind: EventSinkConfig
endpoint: '[fdae:41e4:649b:9303::1]:8091'
---
apiVersion: v1alpha1
kind: KmsgLogConfig
name: omni-kmsg
url: tcp://[fdae:41e4:649b:9303::1]:8092
`, expectedToken)

	expectedKernelArgs := []string{
		fmt.Sprintf("siderolink.api=http://127.0.0.1?jointoken=%s", expectedToken),
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
