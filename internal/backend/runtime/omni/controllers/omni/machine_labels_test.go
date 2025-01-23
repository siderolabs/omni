// Copyright (c) 2025 Sidero Labs, Inc.
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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
)

type MachineLabelsSuite struct {
	OmniSuite
}

func (suite *MachineLabelsSuite) TestLabelsReconcile() {
	require := suite.Require()

	suite.ctx, suite.ctxCancel = context.WithTimeout(suite.ctx, time.Second*5)

	suite.startRuntime()

	require.NoError(suite.runtime.RegisterQController(omnictrl.NewClusterMachineStatusController()))

	machineStatus := omni.NewMachineStatus(
		resources.DefaultNamespace, "machine",
	)

	require.NoError(suite.state.Create(suite.ctx, machineStatus))

	labels := omni.NewMachineLabels(resources.DefaultNamespace, "machine")
	labels.Metadata().Labels().Set("my-awesome-label", "value")

	require.NoError(suite.state.Create(suite.ctx, labels))

	assertResource(
		&suite.OmniSuite,
		machineStatus.Metadata(),
		func(res *omni.MachineStatus, assertions *assert.Assertions) {
			for _, label := range res.Metadata().Labels().Keys() {
				actual, exists := res.Metadata().Labels().Get(label)
				expected, _ := labels.Metadata().Labels().Get(label)

				assertions.True(exists)
				assertions.Equal(expected, actual)
			}
		})

	rtestutils.Destroy[*omni.MachineLabels](suite.ctx, suite.T(), suite.state, []resource.ID{labels.Metadata().ID()})
	assertNoResource(&suite.OmniSuite, labels)

	assertResource(
		&suite.OmniSuite,
		machineStatus.Metadata(),
		func(res *omni.MachineStatus, assertions *assert.Assertions) {
			assertions.True(res.Metadata().Labels().Empty())
		})
}

func TestMachineLabelsSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(MachineLabelsSuite))
}
