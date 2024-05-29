// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"context"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
)

type MaintenanceConfigPatchSuite struct {
	OmniSuite
}

func (suite *MaintenanceConfigPatchSuite) TestReconcile() {
	suite.startRuntime()

	ctx, cancel := context.WithTimeout(suite.ctx, time.Second*5)
	defer cancel()

	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewMaintenanceConfigPatchController()))

	machineID := "test-machine"

	machineStatus := omni.NewMachineStatus(resources.DefaultNamespace, machineID)

	machineStatus.TypedSpec().Value.TalosVersion = "v1.5.0"

	connectionParams := siderolink.NewConnectionParams(resources.DefaultNamespace, siderolink.ConfigID)
	connectionParams.TypedSpec().Value.ApiEndpoint = "grpc://127.0.0.1:8080"
	connectionParams.TypedSpec().Value.JoinToken = "tttt"

	suite.Require().NoError(suite.state.Create(ctx, connectionParams))
	suite.Require().NoError(suite.state.Create(ctx, machineStatus))

	expectedID := omnictrl.MaintenanceConfigPatchPrefix + machineID

	expectedConfig := `apiVersion: v1alpha1
kind: SideroLinkConfig
apiUrl: grpc://127.0.0.1:8080?grpc_tunnel=false&jointoken=tttt
---
apiVersion: v1alpha1
kind: EventSinkConfig
endpoint: '[fdae:41e4:649b:9303::1]:8090'
---
apiVersion: v1alpha1
kind: KmsgLogConfig
name: omni-kmsg
url: tcp://[fdae:41e4:649b:9303::1]:8092
`

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{expectedID},
		func(cp *omni.ConfigPatch, assert *assert.Assertions) {
			assert.Equal(machineID, cp.Metadata().Labels().Raw()[omni.LabelMachine])
			assert.Equal(expectedConfig, cp.TypedSpec().Value.Data)
		},
	)

	_, err := safe.StateUpdateWithConflicts(ctx, suite.state, machineStatus.Metadata(), func(ms *omni.MachineStatus) error {
		ms.TypedSpec().Value.TalosVersion = "v1.4.0"

		return nil
	})
	suite.Require().NoError(err)

	rtestutils.AssertNoResource[*omni.ConfigPatch](ctx, suite.T(), suite.state, expectedID)

	rtestutils.Destroy[*omni.MachineStatus](ctx, suite.T(), suite.state, []resource.ID{machineID})

	rtestutils.AssertNoResource[*omni.ConfigPatch](ctx, suite.T(), suite.state, expectedID)
}

func TestMaintenanceConfigPatchSuite(t *testing.T) {
	suite.Run(t, new(MaintenanceConfigPatchSuite))
}
