// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
)

type MachineClassPatchSuite struct {
	OmniSuite
}

func (suite *MachineClassPatchSuite) TestReconcile() {
	ctx, cancel := context.WithTimeout(suite.ctx, time.Second*5)
	defer cancel()

	suite.startRuntime()

	controller := omnictrl.NewMachineClassPatchController()

	suite.Require().NoError(suite.runtime.RegisterQController(controller))

	machineStatus := omni.NewMachineStatus(resources.DefaultNamespace, "1")
	suite.Require().NoError(suite.state.Create(ctx, machineStatus))

	className := "woopwoop"
	labelName := "test"

	patch := omni.NewConfigPatch(resources.DefaultNamespace, "test")

	patch.Metadata().Labels().Set(omni.LabelMachineClass, className)

	patch.TypedSpec().Value.Data = "patchContents"

	suite.Require().NoError(suite.state.Create(ctx, patch))

	machineClass := omni.NewMachineClass(resources.DefaultNamespace, className)
	machineClass.TypedSpec().Value.MatchLabels = []string{
		labelName,
	}

	suite.Require().NoError(suite.state.Create(ctx, machineClass))

	expectedPatchName := fmt.Sprintf("%s-%s", patch.Metadata().ID(), machineStatus.Metadata().ID())

	rtestutils.AssertNoResource[*omni.ConfigPatch](ctx, suite.T(), suite.state, expectedPatchName)

	_, err := safe.StateUpdateWithConflicts(ctx, suite.state, machineStatus.Metadata(), func(r *omni.MachineStatus) error {
		r.Metadata().Labels().Set(labelName, "")

		return nil
	})
	suite.Require().NoError(err)

	rtestutils.AssertNoResource[*omni.ConfigPatch](ctx, suite.T(), suite.state, expectedPatchName)

	machineStatus, err = safe.StateUpdateWithConflicts(ctx, suite.state, machineStatus.Metadata(), func(r *omni.MachineStatus) error {
		r.Metadata().Labels().Set(omni.LabelCluster, "cluster")

		return nil
	})
	suite.Require().NoError(err)

	assertPatchExists := func() {
		rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{expectedPatchName}, func(r *omni.ConfigPatch, assertion *assert.Assertions) {
			assertion.Equal(patch.TypedSpec().Value, r.TypedSpec().Value)

			source, ok := r.Metadata().Labels().Get(omni.LabelConfigPatchClass)
			assertion.True(ok)
			assertion.Equal(patch.Metadata().ID(), source)

			target, ok := r.Metadata().Labels().Get(omni.LabelClusterMachineClassPatch)
			assertion.True(ok)
			assertion.Equal(machineStatus.Metadata().ID(), target)

			_, ok = r.Metadata().Labels().Get(omni.LabelCluster)
			assertion.True(ok)
		})
	}

	assertPatchExists()

	rtestutils.Destroy[*omni.MachineClass](ctx, suite.T(), suite.state, []string{machineClass.Metadata().ID()})

	rtestutils.AssertNoResource[*omni.ConfigPatch](ctx, suite.T(), suite.state, expectedPatchName)

	suite.Require().NoError(suite.state.Create(ctx, machineClass))

	assertPatchExists()

	rtestutils.Destroy[*omni.MachineStatus](ctx, suite.T(), suite.state, []string{machineStatus.Metadata().ID()})

	rtestutils.AssertNoResource[*omni.ConfigPatch](ctx, suite.T(), suite.state, expectedPatchName)

	suite.Require().NoError(suite.state.Create(ctx, machineStatus))

	assertPatchExists()

	rtestutils.Destroy[*omni.ConfigPatch](ctx, suite.T(), suite.state, []string{patch.Metadata().ID()})

	rtestutils.AssertNoResource[*omni.ConfigPatch](ctx, suite.T(), suite.state, expectedPatchName)

	suite.Require().NoError(suite.state.Create(ctx, patch))

	assertPatchExists()

	machineStatus, err = safe.StateUpdateWithConflicts(ctx, suite.state, machineStatus.Metadata(), func(r *omni.MachineStatus) error {
		r.Metadata().Labels().Delete(omni.LabelCluster)

		return nil
	})
	suite.Require().NoError(err)

	rtestutils.AssertNoResource[*omni.ConfigPatch](ctx, suite.T(), suite.state, expectedPatchName)
}

func TestMachineClassPatchSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(MachineClassPatchSuite))
}
