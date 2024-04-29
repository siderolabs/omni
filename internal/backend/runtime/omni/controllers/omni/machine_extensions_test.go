// Copyright (c) 2024 Sidero Labs, Inc.
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
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
)

type ExtensionsConfigurationStatusSuite struct {
	OmniSuite
}

func (suite *ExtensionsConfigurationStatusSuite) TestReconcile() {
	suite.startRuntime()

	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewMachineExtensionsController()))

	machine := omni.NewClusterMachine(resources.DefaultNamespace, "test")
	machine.Metadata().Labels().Set(omni.LabelCluster, "cluster")
	machine.Metadata().Labels().Set(omni.LabelMachineSet, "machineSet")

	clusterSchematic := omni.NewExtensionsConfiguration(resources.DefaultNamespace, "aaa")
	clusterSchematic.Metadata().Labels().Set(omni.LabelCluster, "cluster")

	someOtherMachineSchematic := omni.NewExtensionsConfiguration(resources.DefaultNamespace, "aaa")
	someOtherMachineSchematic.Metadata().Labels().Set(omni.LabelCluster, "cluster")
	someOtherMachineSchematic.Metadata().Labels().Set(omni.LabelClusterMachine, "bbb")

	thisMachineSchematic := omni.NewExtensionsConfiguration(resources.DefaultNamespace, "aaa")
	thisMachineSchematic.Metadata().Labels().Set(omni.LabelCluster, "cluster")
	thisMachineSchematic.Metadata().Labels().Set(omni.LabelClusterMachine, "test")

	someOtherMachineSet := omni.NewExtensionsConfiguration(resources.DefaultNamespace, "aaa")
	someOtherMachineSet.Metadata().Labels().Set(omni.LabelCluster, "cluster")
	someOtherMachineSet.Metadata().Labels().Set(omni.LabelMachineSet, "aaa")

	thisMachineSet := omni.NewExtensionsConfiguration(resources.DefaultNamespace, "aaa")
	thisMachineSet.Metadata().Labels().Set(omni.LabelCluster, "cluster")
	thisMachineSet.Metadata().Labels().Set(omni.LabelMachineSet, "machineSet")

	extensions := []string{"zzzz"}

	for _, tt := range []struct {
		name               string
		extensions         *omni.ExtensionsConfiguration
		machine            *omni.ClusterMachine
		expectedExtensions []string
	}{
		{
			name:       "empty",
			extensions: omni.NewExtensionsConfiguration(resources.DefaultNamespace, "aaa"),
			machine:    machine,
		},
		{
			name:               "defined for a cluster",
			extensions:         clusterSchematic,
			machine:            machine,
			expectedExtensions: extensions,
		},
		{
			name:       "defined for a differrent machine",
			extensions: someOtherMachineSchematic,
			machine:    machine,
		},
		{
			name:               "defined for this machine",
			extensions:         thisMachineSchematic,
			machine:            machine,
			expectedExtensions: extensions,
		},
		{
			name:       "defined for other machine set",
			extensions: someOtherMachineSet,
			machine:    machine,
		},
		{
			name:               "defined for other this machine set",
			extensions:         thisMachineSet,
			machine:            machine,
			expectedExtensions: extensions,
		},
	} {
		suite.T().Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(suite.ctx, time.Second*3)
			defer cancel()

			machineStatus := omni.NewMachineStatus(resources.DefaultNamespace, machine.Metadata().ID())

			tt.extensions.TypedSpec().Value.Extensions = extensions

			require := require.New(t)

			require.NoError(suite.state.Create(ctx, machineStatus))
			require.NoError(suite.state.Create(ctx, machine))
			require.NoError(suite.state.Create(ctx, tt.extensions))

			rtestutils.AssertResource[*omni.ExtensionsConfiguration](ctx, t, suite.state, tt.extensions.Metadata().ID(), func(r *omni.ExtensionsConfiguration, assertion *assert.Assertions) {
				assertion.True(r.Metadata().Finalizers().Has(omnictrl.MachineExtensionsControllerName))
			})

			defer func() {
				rtestutils.DestroyAll[*omni.MachineStatus](ctx, t, suite.state)
				rtestutils.DestroyAll[*omni.ClusterMachine](ctx, t, suite.state)
				rtestutils.DestroyAll[*omni.ExtensionsConfiguration](ctx, t, suite.state)
			}()

			if tt.expectedExtensions != nil {
				rtestutils.AssertResources[*omni.MachineExtensions](ctx, t, suite.state, []string{machine.Metadata().ID()}, func(r *omni.MachineExtensions, assertion *assert.Assertions) {
					assertion.Equal(tt.expectedExtensions, r.TypedSpec().Value.Extensions)
				})
			} else {
				rtestutils.AssertNoResource[*omni.MachineExtensions](ctx, t, suite.state, machine.Metadata().ID())
			}
		})
	}
}

func TestExtensionsConfigurationStatusSuite(t *testing.T) {
	suite.Run(t, new(ExtensionsConfigurationStatusSuite))
}
