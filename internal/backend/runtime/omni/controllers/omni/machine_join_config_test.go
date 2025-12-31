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

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
)

type MachineJoinConfigSuite struct {
	OmniSuite
}

func (suite *MachineJoinConfigSuite) TestReconcile() {
	suite.startRuntime()

	ctx, cancel := context.WithTimeout(suite.ctx, time.Second*5)

	defer cancel()

	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewMachineJoinConfigController()))

	apiConfig := siderolink.NewAPIConfig()
	apiConfig.TypedSpec().Value.EventsPort = 8091
	apiConfig.TypedSpec().Value.LogsPort = 8092
	apiConfig.TypedSpec().Value.MachineApiAdvertisedUrl = "http://127.0.0.1"

	machine := omni.NewMachine("machine1")

	params := siderolink.NewJoinTokenUsage(machine.Metadata().ID())
	params.TypedSpec().Value.TokenId = "jtoken"

	suite.Require().NoError(suite.state.Create(suite.ctx, params))
	suite.Require().NoError(suite.state.Create(suite.ctx, apiConfig))
	suite.Require().NoError(suite.state.Create(suite.ctx, machine))

	expectedConfig := `apiVersion: v1alpha1
kind: SideroLinkConfig
apiUrl: http://127.0.0.1?jointoken=jtoken
---
apiVersion: v1alpha1
kind: EventSinkConfig
endpoint: '[fdae:41e4:649b:9303::1]:8091'
---
apiVersion: v1alpha1
kind: KmsgLogConfig
name: omni-kmsg
url: tcp://[fdae:41e4:649b:9303::1]:8092
`

	expectedKernelArgs := []string{
		"siderolink.api=http://127.0.0.1?jointoken=jtoken",
		"talos.events.sink=[fdae:41e4:649b:9303::1]:8091",
		"talos.logging.kernel=tcp://[fdae:41e4:649b:9303::1]:8092",
	}

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{machine.Metadata().ID()},
		func(res *siderolink.MachineJoinConfig, assert *assert.Assertions) {
			assert.Equal(expectedConfig, res.TypedSpec().Value.Config.Config)
			assert.Equal(expectedKernelArgs, res.TypedSpec().Value.Config.KernelArgs)
		},
	)
}

func TestMachineJoinConfigSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(MachineJoinConfigSuite))
}
