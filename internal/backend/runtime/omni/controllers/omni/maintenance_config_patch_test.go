// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"testing"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
)

type MaintenanceConfigPatchSuite struct {
	OmniSuite
}

func (suite *MaintenanceConfigPatchSuite) TestReconcile() {
	suite.startRuntime()

	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewMaintenanceConfigPatchController()))

	machineID := "test-machine"

	machineStatus := omni.NewMachineStatus(resources.DefaultNamespace, machineID)

	machineStatus.TypedSpec().Value.MaintenanceConfig = &specs.MachineStatusSpec_MaintenanceConfig{
		Config: "# some config",
	}

	suite.Require().NoError(suite.state.Create(suite.ctx, machineStatus))

	expectedID := omnictrl.MaintenanceConfigPatchPrefix + machineID

	rtestutils.AssertResources(suite.ctx, suite.T(), suite.state, []string{expectedID},
		func(cp *omni.ConfigPatch, assert *assert.Assertions) {
			assert.Equal(machineID, cp.Metadata().Labels().Raw()[omni.LabelMachine])
			assert.Equal(machineStatus.TypedSpec().Value.GetMaintenanceConfig().GetConfig(), cp.TypedSpec().Value.Data)
		},
	)

	_, err := safe.StateUpdateWithConflicts(suite.ctx, suite.state, machineStatus.Metadata(), func(ms *omni.MachineStatus) error {
		ms.TypedSpec().Value.MaintenanceConfig.Config = "# some other config"

		return nil
	})
	suite.Require().NoError(err)

	rtestutils.AssertResources(suite.ctx, suite.T(), suite.state, []string{expectedID}, func(cp *omni.ConfigPatch, assert *assert.Assertions) {
		assert.Equal(machineStatus.TypedSpec().Value.GetMaintenanceConfig().GetConfig(), cp.TypedSpec().Value.Data)
	})

	rtestutils.Destroy[*omni.MachineStatus](suite.ctx, suite.T(), suite.state, []resource.ID{machineID})

	rtestutils.AssertNoResource[*omni.ConfigPatch](suite.ctx, suite.T(), suite.state, expectedID)
}

func TestMaintenanceConfigPatchSuite(t *testing.T) {
	suite.Run(t, new(MaintenanceConfigPatchSuite))
}
