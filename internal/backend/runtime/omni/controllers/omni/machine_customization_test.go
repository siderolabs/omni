// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"context"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/controller/runtime"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
)

func TestExtensionsConfigurationStatus(t *testing.T) {
	t.Parallel()

	sb := dynamicStateBuilder{m: map[resource.Namespace]state.CoreState{}}

	withRuntime(
		t.Context(),
		t,
		sb.Builder,
		func(_ context.Context, _ state.State, rt *runtime.Runtime, _ *zap.Logger) { // prepare - register controllers
			require.NoError(t, rt.RegisterQController(omnictrl.NewMachineExtensionsController()))
		},
		func(ctx context.Context, st state.State, _ *runtime.Runtime, _ *zap.Logger) {
			testExtensionsConfigurationStatus(ctx, t, st)
		},
	)
}

//nolint:dupl
func testExtensionsConfigurationStatus(rootCtx context.Context, t *testing.T, st state.State) {
	machine := omni.NewClusterMachine(resources.DefaultNamespace, "test")
	machine.Metadata().Labels().Set(omni.LabelCluster, "cluster")
	machine.Metadata().Labels().Set(omni.LabelMachineSet, "machineSet")

	config := omni.NewExtensionsConfiguration(resources.DefaultNamespace, "aaa")
	config.Metadata().Labels().Set(omni.LabelCluster, "cluster")

	someOtherMachineConfig := omni.NewExtensionsConfiguration(resources.DefaultNamespace, "aaa")
	someOtherMachineConfig.Metadata().Labels().Set(omni.LabelCluster, "cluster")
	someOtherMachineConfig.Metadata().Labels().Set(omni.LabelClusterMachine, "bbb")

	thisMachineConfig := omni.NewExtensionsConfiguration(resources.DefaultNamespace, "aaa")
	thisMachineConfig.Metadata().Labels().Set(omni.LabelCluster, "cluster")
	thisMachineConfig.Metadata().Labels().Set(omni.LabelClusterMachine, "test")

	someOtherMachineSetConfig := omni.NewExtensionsConfiguration(resources.DefaultNamespace, "aaa")
	someOtherMachineSetConfig.Metadata().Labels().Set(omni.LabelCluster, "cluster")
	someOtherMachineSetConfig.Metadata().Labels().Set(omni.LabelMachineSet, "aaa")

	thisMachineSetConfig := omni.NewExtensionsConfiguration(resources.DefaultNamespace, "aaa")
	thisMachineSetConfig.Metadata().Labels().Set(omni.LabelCluster, "cluster")
	thisMachineSetConfig.Metadata().Labels().Set(omni.LabelMachineSet, "machineSet")

	configVal := []string{"zzzz"}

	for _, tt := range []struct {
		name               string
		config             *omni.ExtensionsConfiguration
		machine            *omni.ClusterMachine
		expectedExtensions []string
	}{
		{
			name:    "empty",
			config:  omni.NewExtensionsConfiguration(resources.DefaultNamespace, "aaa"),
			machine: machine,
		},
		{
			name:               "defined for a cluster",
			config:             config,
			machine:            machine,
			expectedExtensions: configVal,
		},
		{
			name:    "defined for a different machine",
			config:  someOtherMachineConfig,
			machine: machine,
		},
		{
			name:               "defined for this machine",
			config:             thisMachineConfig,
			machine:            machine,
			expectedExtensions: configVal,
		},
		{
			name:    "defined for other machine set",
			config:  someOtherMachineSetConfig,
			machine: machine,
		},
		{
			name:               "defined for other this machine set",
			config:             thisMachineSetConfig,
			machine:            machine,
			expectedExtensions: configVal,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(rootCtx, time.Second*3)
			defer cancel()

			machineStatus := omni.NewMachineStatus(resources.DefaultNamespace, machine.Metadata().ID())

			tt.config.TypedSpec().Value.Extensions = configVal

			require := require.New(t)

			require.NoError(st.Create(ctx, machineStatus))
			require.NoError(st.Create(ctx, machine))
			require.NoError(st.Create(ctx, tt.config))

			rtestutils.AssertResource[*omni.ExtensionsConfiguration](ctx, t, st, tt.config.Metadata().ID(), func(r *omni.ExtensionsConfiguration, assertion *assert.Assertions) {
				assertion.True(r.Metadata().Finalizers().Has(omnictrl.MachineExtensionsControllerName))
			})

			defer func() {
				rtestutils.DestroyAll[*omni.MachineStatus](ctx, t, st)
				rtestutils.DestroyAll[*omni.ClusterMachine](ctx, t, st)
				rtestutils.DestroyAll[*omni.ExtensionsConfiguration](ctx, t, st)
			}()

			if tt.expectedExtensions != nil {
				rtestutils.AssertResources[*omni.MachineExtensions](ctx, t, st, []string{machine.Metadata().ID()}, func(r *omni.MachineExtensions, assertion *assert.Assertions) {
					assertion.Equal(tt.expectedExtensions, r.TypedSpec().Value.Extensions)
				})
			} else {
				rtestutils.AssertNoResource[*omni.MachineExtensions](ctx, t, st, machine.Metadata().ID())
			}
		})
	}
}

func TestExtraKernelArgsConfigurationStatus(t *testing.T) {
	t.Parallel()

	sb := dynamicStateBuilder{m: map[resource.Namespace]state.CoreState{}}

	withRuntime(
		t.Context(),
		t,
		sb.Builder,
		func(_ context.Context, _ state.State, rt *runtime.Runtime, _ *zap.Logger) { // prepare - register controllers
			require.NoError(t, rt.RegisterQController(omnictrl.NewMachineExtraKernelArgsController()))
		},
		func(ctx context.Context, st state.State, _ *runtime.Runtime, _ *zap.Logger) {
			testExtraKernelArgsConfigurationStatus(ctx, t, st)
		},
	)
}

//nolint:dupl
func testExtraKernelArgsConfigurationStatus(rootCtx context.Context, t *testing.T, st state.State) {
	machine := omni.NewClusterMachine(resources.DefaultNamespace, "test")
	machine.Metadata().Labels().Set(omni.LabelCluster, "cluster")
	machine.Metadata().Labels().Set(omni.LabelMachineSet, "machineSet")

	config := omni.NewExtraKernelArgsConfiguration(resources.DefaultNamespace, "aaa")
	config.Metadata().Labels().Set(omni.LabelCluster, "cluster")

	someOtherMachineConfig := omni.NewExtraKernelArgsConfiguration(resources.DefaultNamespace, "aaa")
	someOtherMachineConfig.Metadata().Labels().Set(omni.LabelCluster, "cluster")
	someOtherMachineConfig.Metadata().Labels().Set(omni.LabelClusterMachine, "bbb")

	thisMachineConfig := omni.NewExtraKernelArgsConfiguration(resources.DefaultNamespace, "aaa")
	thisMachineConfig.Metadata().Labels().Set(omni.LabelCluster, "cluster")
	thisMachineConfig.Metadata().Labels().Set(omni.LabelClusterMachine, "test")

	someOtherMachineSetConfig := omni.NewExtraKernelArgsConfiguration(resources.DefaultNamespace, "aaa")
	someOtherMachineSetConfig.Metadata().Labels().Set(omni.LabelCluster, "cluster")
	someOtherMachineSetConfig.Metadata().Labels().Set(omni.LabelMachineSet, "aaa")

	thisMachineSetConfig := omni.NewExtraKernelArgsConfiguration(resources.DefaultNamespace, "aaa")
	thisMachineSetConfig.Metadata().Labels().Set(omni.LabelCluster, "cluster")
	thisMachineSetConfig.Metadata().Labels().Set(omni.LabelMachineSet, "machineSet")

	configVal := []string{"zzzz"}

	for _, tt := range []struct {
		name     string
		config   *omni.ExtraKernelArgsConfiguration
		machine  *omni.ClusterMachine
		expected []string
	}{
		{
			name:    "empty",
			config:  omni.NewExtraKernelArgsConfiguration(resources.DefaultNamespace, "aaa"),
			machine: machine,
		},
		{
			name:     "defined for a cluster",
			config:   config,
			machine:  machine,
			expected: configVal,
		},
		{
			name:    "defined for a different machine",
			config:  someOtherMachineConfig,
			machine: machine,
		},
		{
			name:     "defined for this machine",
			config:   thisMachineConfig,
			machine:  machine,
			expected: configVal,
		},
		{
			name:    "defined for other machine set",
			config:  someOtherMachineSetConfig,
			machine: machine,
		},
		{
			name:     "defined for other this machine set",
			config:   thisMachineSetConfig,
			machine:  machine,
			expected: configVal,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(rootCtx, time.Second*3)
			defer cancel()

			machineStatus := omni.NewMachineStatus(resources.DefaultNamespace, machine.Metadata().ID())

			tt.config.TypedSpec().Value.Args = configVal

			require := require.New(t)

			require.NoError(st.Create(ctx, machineStatus))
			require.NoError(st.Create(ctx, machine))
			require.NoError(st.Create(ctx, tt.config))

			rtestutils.AssertResource[*omni.ExtraKernelArgsConfiguration](ctx, t, st, tt.config.Metadata().ID(), func(r *omni.ExtraKernelArgsConfiguration, assertion *assert.Assertions) {
				assertion.True(r.Metadata().Finalizers().Has(omnictrl.MachineExtraKernelArgsControllerName))
			})

			defer func() {
				rtestutils.DestroyAll[*omni.MachineStatus](ctx, t, st)
				rtestutils.DestroyAll[*omni.ClusterMachine](ctx, t, st)
				rtestutils.DestroyAll[*omni.ExtraKernelArgsConfiguration](ctx, t, st)
			}()

			if tt.expected != nil {
				rtestutils.AssertResources[*omni.MachineExtraKernelArgs](ctx, t, st, []string{machine.Metadata().ID()}, func(r *omni.MachineExtraKernelArgs, assertion *assert.Assertions) {
					assertion.Equal(tt.expected, r.TypedSpec().Value.Args)
				})
			} else {
				rtestutils.AssertNoResource[*omni.MachineExtraKernelArgs](ctx, t, st, machine.Metadata().ID())
			}
		})
	}
}
