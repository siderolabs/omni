// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package kernelargs_test

import (
	"context"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	talosconstants "github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/kernelargs"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils"
)

func TestReconcile(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	testutils.WithRuntime(ctx, t, testutils.TestOptions{}, func(ctx context.Context, testContext testutils.TestContext) {
		require.NoError(t, testContext.Runtime.RegisterQController(kernelargs.NewStatusController()))
	}, func(ctx context.Context, testContext testutils.TestContext) {
		const id = "test"

		ms := omni.NewMachineStatus(resources.DefaultNamespace, id)
		ms.TypedSpec().Value.KernelCmdline = "current cmdline"

		require.NoError(t, testContext.State.Create(ctx, ms))

		rtestutils.AssertResource(ctx, t, testContext.State, id, func(res *omni.KernelArgsStatus, assertion *assert.Assertions) {
			assertion.Equal([]string{
				"Schematic information is not yet known",
				"Talos is not installed, kernel args cannot be updated yet",
				"Cannot determine if kernel args update is supported: SecurityState and TalosVersion are not yet set",
			}, res.TypedSpec().Value.UnmetConditions)
			assert.Equal(t, "current cmdline", res.TypedSpec().Value.CurrentCmdline)
		})

		initialKernelArgs := []string{"arg-1", "arg-2"}

		_, err := safe.StateUpdateWithConflicts(ctx, testContext.State, ms.Metadata(), func(res *omni.MachineStatus) error {
			res.TypedSpec().Value.TalosVersion = "v1.11.3"

			res.TypedSpec().Value.Hardware = &specs.MachineStatusSpec_HardwareStatus{
				Blockdevices: []*specs.MachineStatusSpec_HardwareStatus_BlockDevice{{LinuxName: "/dev/sda", SystemDisk: true}},
			}

			res.TypedSpec().Value.PlatformMetadata = &specs.MachineStatusSpec_PlatformMetadata{
				Platform: talosconstants.PlatformMetal,
			}

			res.TypedSpec().Value.Schematic = &specs.MachineStatusSpec_Schematic{
				KernelArgs: []string{"arg-1", "arg-2"},
			}

			return nil
		})
		require.NoError(t, err)

		rtestutils.AssertResource(ctx, t, testContext.State, id, func(res *omni.KernelArgsStatus, assertion *assert.Assertions) {
			assertion.Equal([]string{
				"Unsupported: machine is not booted with UKI and Talos version is < 1.12 or GrubUseUKICmdline is false",
				"Extra kernel args are not yet initialized",
			}, res.TypedSpec().Value.UnmetConditions)
		})

		_, err = safe.StateUpdateWithConflicts(ctx, testContext.State, ms.Metadata(), func(res *omni.MachineStatus) error {
			res.Metadata().Annotations().Set(omni.KernelArgsInitialized, "")
			res.TypedSpec().Value.SecurityState = &specs.SecurityState{BootedWithUki: true}

			return nil
		})
		require.NoError(t, err)

		rtestutils.AssertResource(ctx, t, testContext.State, id, func(res *omni.KernelArgsStatus, assertion *assert.Assertions) {
			assertion.Empty(res.TypedSpec().Value.UnmetConditions)
			assertion.Equal(initialKernelArgs, res.TypedSpec().Value.CurrentArgs)
			assertion.Empty(res.TypedSpec().Value.Args)
			assert.Equal(t, "current cmdline", res.TypedSpec().Value.CurrentCmdline)
		})

		// Make the machine non-UKI, add the machine to a cluster with GrubUseUkiCmdline set to false in its config.
		// Assert that the machine does not support updating the kernel args.
		cmc := omni.NewClusterMachineConfig(resources.DefaultNamespace, id)
		require.NoError(t, testContext.State.Create(ctx, cmc))

		_, err = safe.StateUpdateWithConflicts(ctx, testContext.State, ms.Metadata(), func(res *omni.MachineStatus) error {
			res.Metadata().Annotations().Set(omni.KernelArgsInitialized, "")
			res.TypedSpec().Value.TalosVersion = "1.12.0"
			res.TypedSpec().Value.SecurityState = &specs.SecurityState{BootedWithUki: false}

			return nil
		})
		require.NoError(t, err)

		rtestutils.AssertResource(ctx, t, testContext.State, id, func(res *omni.KernelArgsStatus, assertion *assert.Assertions) {
			assertion.Equal([]string{
				"Unsupported: machine is not booted with UKI and Talos version is < 1.12 or GrubUseUKICmdline is false",
			}, res.TypedSpec().Value.UnmetConditions)
		})

		// Update the cluster config to set GrubUseUkiCmdline to true. Assert that the machine now supports updating the kernel args, despite being non-UKI.
		cmc.TypedSpec().Value.GrubUseUkiCmdline = true
		require.NoError(t, testContext.State.Update(ctx, cmc))

		args := omni.NewKernelArgs(id)
		args.TypedSpec().Value.Args = []string{"updated-arg-1", "updated-arg-2"}

		require.NoError(t, testContext.State.Create(ctx, args))

		rtestutils.AssertResource(ctx, t, testContext.State, id, func(res *omni.KernelArgsStatus, assertion *assert.Assertions) {
			assertion.Empty(res.TypedSpec().Value.UnmetConditions)
			assertion.Equal(initialKernelArgs, res.TypedSpec().Value.CurrentArgs)
			assertion.Equal([]string{"updated-arg-1", "updated-arg-2"}, res.TypedSpec().Value.Args)
		})
	})
}
