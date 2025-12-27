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
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils"
)

type MachineExtensionsSuite struct {
	OmniSuite
}

func (suite *MachineExtensionsSuite) TestReconcile() {
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
			name:       "defined for a different machine",
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

func TestMachineExtensionsSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(MachineExtensionsSuite))
}

func TestMachineExtensionsPriority(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	testutils.WithRuntime(ctx, t, testutils.TestOptions{}, func(ctx context.Context, testContext testutils.TestContext) {
		clusterLevelConfig := omni.NewExtensionsConfiguration(resources.DefaultNamespace, "cluster-level")
		clusterLevelConfig.Metadata().Labels().Set(omni.LabelCluster, "cluster")
		clusterLevelConfig.TypedSpec().Value.Extensions = []string{"cluster-level"}

		machineSetLevelConfig := omni.NewExtensionsConfiguration(resources.DefaultNamespace, "machine-set-level")
		machineSetLevelConfig.Metadata().Labels().Set(omni.LabelCluster, "cluster")
		machineSetLevelConfig.Metadata().Labels().Set(omni.LabelMachineSet, "machine-set")
		machineSetLevelConfig.TypedSpec().Value.Extensions = []string{"machine-set-level"}

		clusterMachineLevel1 := omni.NewExtensionsConfiguration(resources.DefaultNamespace, "cluster-machine-level-1")
		clusterMachineLevel1.Metadata().Labels().Set(omni.LabelCluster, "cluster")
		clusterMachineLevel1.Metadata().Labels().Set(omni.LabelClusterMachine, "cluster-machine")
		clusterMachineLevel1.TypedSpec().Value.Extensions = []string{"cluster-machine-level-1"}

		clusterMachineLevel2 := omni.NewExtensionsConfiguration(resources.DefaultNamespace, "cluster-machine-level-2")
		clusterMachineLevel2.Metadata().Labels().Set(omni.LabelCluster, "cluster")
		clusterMachineLevel2.Metadata().Labels().Set(omni.LabelClusterMachine, "cluster-machine")
		clusterMachineLevel2.TypedSpec().Value.Extensions = []string{"cluster-machine-level-2"}

		st := testContext.State

		require.NoError(t, st.Create(ctx, clusterLevelConfig))
		require.NoError(t, st.Create(ctx, machineSetLevelConfig))
		require.NoError(t, st.Create(ctx, clusterMachineLevel1))
		require.NoError(t, st.Create(ctx, clusterMachineLevel2))

		clusterMachine := omni.NewClusterMachine(resources.DefaultNamespace, "cluster-machine")
		clusterMachine.Metadata().Labels().Set(omni.LabelCluster, "cluster")
		clusterMachine.Metadata().Labels().Set(omni.LabelMachineSet, "machine-set")

		require.NoError(t, st.Create(ctx, clusterMachine))

		controller := omnictrl.NewMachineExtensionsController()
		require.NoError(t, testContext.Runtime.RegisterQController(controller))
	}, func(ctx context.Context, testContext testutils.TestContext) {
		rtestutils.AssertResource(ctx, t, testContext.State, "cluster-machine", func(res *omni.MachineExtensions, assertion *assert.Assertions) {
			assertion.Equal([]string{"cluster-machine-level-2"}, res.TypedSpec().Value.Extensions)
		})
	})
}

func TestPreserveLegacyOrder(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	testutils.WithRuntime(ctx, t, testutils.TestOptions{}, func(ctx context.Context, testContext testutils.TestContext) {
		clusterLevelConfig := omni.NewExtensionsConfiguration(resources.DefaultNamespace, "cluster-level")
		clusterLevelConfig.Metadata().Finalizers().Add(omnictrl.MachineExtensionsControllerName)
		clusterLevelConfig.Metadata().Labels().Set(omni.LabelCluster, "cluster")
		clusterLevelConfig.TypedSpec().Value.Extensions = []string{"cluster-level"}

		machineSetLevelConfig := omni.NewExtensionsConfiguration(resources.DefaultNamespace, "machine-set-level")
		machineSetLevelConfig.Metadata().Finalizers().Add(omnictrl.MachineExtensionsControllerName)
		machineSetLevelConfig.Metadata().Labels().Set(omni.LabelCluster, "cluster")
		machineSetLevelConfig.Metadata().Labels().Set(omni.LabelMachineSet, "machine-set")
		machineSetLevelConfig.TypedSpec().Value.Extensions = []string{"machine-set-level"}

		clusterMachineLevel1 := omni.NewExtensionsConfiguration(resources.DefaultNamespace, "cluster-machine-level-1")
		clusterMachineLevel1.Metadata().Finalizers().Add(omnictrl.MachineExtensionsControllerName)
		clusterMachineLevel1.Metadata().Labels().Set(omni.LabelCluster, "cluster")
		clusterMachineLevel1.Metadata().Labels().Set(omni.LabelClusterMachine, "cluster-machine")
		clusterMachineLevel1.TypedSpec().Value.Extensions = []string{"cluster-machine-level-1"}

		clusterMachineLevel2 := omni.NewExtensionsConfiguration(resources.DefaultNamespace, "cluster-machine-level-2")
		clusterMachineLevel2.Metadata().Finalizers().Add(omnictrl.MachineExtensionsControllerName)
		clusterMachineLevel2.Metadata().Labels().Set(omni.LabelCluster, "cluster")
		clusterMachineLevel2.Metadata().Labels().Set(omni.LabelClusterMachine, "cluster-machine")
		clusterMachineLevel2.TypedSpec().Value.Extensions = []string{"cluster-machine-level-2"}

		st := testContext.State

		require.NoError(t, st.Create(ctx, clusterLevelConfig))
		require.NoError(t, st.Create(ctx, machineSetLevelConfig))
		require.NoError(t, st.Create(ctx, clusterMachineLevel1))
		require.NoError(t, st.Create(ctx, clusterMachineLevel2))

		clusterMachine := omni.NewClusterMachine(resources.DefaultNamespace, "cluster-machine")
		clusterMachine.Metadata().Labels().Set(omni.LabelCluster, "cluster")
		clusterMachine.Metadata().Labels().Set(omni.LabelMachineSet, "machine-set")

		require.NoError(t, st.Create(ctx, clusterMachine))

		// prepare a MachineExtensions with the wrong extension list - assume that it picked the cluster level extensions instead of the cluster machine level ones.
		machineExtensions := omni.NewMachineExtensions(resources.DefaultNamespace, "cluster-machine")
		machineExtensions.TypedSpec().Value.Extensions = []string{"cluster-level"}

		require.NoError(t, st.Create(ctx, machineExtensions, state.WithCreateOwner(omnictrl.MachineExtensionsControllerName)))

		controller := omnictrl.NewMachineExtensionsController()
		require.NoError(t, testContext.Runtime.RegisterQController(controller))
	}, func(ctx context.Context, testContext testutils.TestContext) {
		rtestutils.AssertResource(ctx, t, testContext.State, "cluster-machine", func(res *omni.MachineExtensions, assertion *assert.Assertions) {
			_, annotationOk := res.Metadata().Annotations().Get(helpers.InputResourceVersionAnnotation)
			assertion.True(annotationOk)

			assertion.Equal([]string{"cluster-level"}, res.TypedSpec().Value.Extensions)
		})
	})
}
