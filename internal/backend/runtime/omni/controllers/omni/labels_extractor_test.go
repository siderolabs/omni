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
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/system"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
)

type LabelsExtractorControllerSuite struct {
	OmniSuite
}

func (suite *LabelsExtractorControllerSuite) TestReconcile() {
	suite.startRuntime()

	ctx, cancel := context.WithTimeout(suite.ctx, time.Second*5)
	defer cancel()

	suite.Require().NoError(
		suite.runtime.RegisterQController(omnictrl.NewLabelsExtractorController[*omni.MachineStatus]()),
	)

	clusterName := "testcluster"

	machine := omni.NewMachineStatus("1")
	machine.Metadata().Labels().Set(omni.LabelCluster, clusterName)

	suite.Require().NoError(suite.state.Create(ctx, machine))

	labelsID := machine.Metadata().ID()

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{labelsID},
		func(labels *system.ResourceLabels[*omni.MachineStatus], assert *assert.Assertions) {
			value, ok := labels.Metadata().Labels().Get(omni.LabelCluster)

			assert.True(ok)
			assert.Equal(clusterName, value)
		},
	)

	_, err := safe.StateUpdateWithConflicts(ctx, suite.state, machine.Metadata(), func(res *omni.MachineStatus) error {
		res.Metadata().Labels().Delete(omni.LabelCluster)
		res.Metadata().Labels().Set(omni.MachineStatusLabelAvailable, "")

		return nil
	})
	suite.Require().NoError(err)

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{labelsID},
		func(labels *system.ResourceLabels[*omni.MachineStatus], assert *assert.Assertions) {
			_, ok := labels.Metadata().Labels().Get(omni.LabelCluster)
			assert.False(ok)

			_, ok = labels.Metadata().Labels().Get(omni.MachineStatusLabelAvailable)
			assert.True(ok)
		},
	)

	rtestutils.DestroyAll[*omni.MachineStatus](ctx, suite.T(), suite.state)

	rtestutils.AssertNoResource[*system.ResourceLabels[*omni.MachineStatus]](ctx, suite.T(), suite.state, labelsID)
}

func TestLabelsExtractorControllerSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(LabelsExtractorControllerSuite))
}
