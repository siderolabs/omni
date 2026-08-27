// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build integration

package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// kubeletReservedCPUsPatch pins kubelet system-reserved CPUs to cores 1 and 2. It is valid on the
// 3-CPU test machines, and becomes invalid once the maxcpus=2 kernel arg leaves only CPUs 0-1 online:
// kubelet then fails validation with "reserved-cpus is not a subset of online-cpus" and crashloops,
// exactly the failure mode of siderolabs/omni#3262.
const kubeletReservedCPUsPatch = `machine:
  kubelet:
    extraArgs:
      reserved-cpus: "1,2"`

// testKernelArgsBreakAndRevert breaks a machine through a kernel args change and verifies that reverting
// the change recovers it (siderolabs/omni#3262). The broken machine stays connected to Omni but is not
// healthy, so before the fix neither the revert nor anything else could reach it, and the machine was
// stuck until manual bootloader intervention or a reprovision.
//
// On emulated (talemu) machines the breakage is the talemu.stuck=booting magic arg. On QEMU machines it
// is the real thing: a kubelet reserved-cpus setting that a maxcpus kernel arg invalidates.
func testKernelArgsBreakAndRevert(t *testing.T, options *TestOptions) {
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Minute)
	defer cancel()

	options.claimMachines(t, 2)

	clusterName := "integration-kernel-args-revert"

	t.Run(
		"ClusterShouldBeCreated",
		CreateCluster(t.Context(), options, ClusterOptions{
			Name:          clusterName,
			ControlPlanes: 1,
			Workers:       1,

			MachineOptions: options.MachineOptions,
			ScalingTimeout: options.ScalingTimeout,

			SkipExtensionCheckOnCreate: options.SkipExtensionsCheckOnCreate,
		}),
	)

	assertClusterAndAPIReady(t, clusterName, options)

	omniState := options.omniClient.Omni().State()

	workerIDs := rtestutils.ResourceIDs[*omni.ClusterMachine](ctx, t, omniState, state.WithLabelQuery(
		resource.LabelEqual(omni.LabelCluster, clusterName),
		resource.LabelExists(omni.LabelWorkerRole),
	))
	require.Len(t, workerIDs, 1)

	machineID := workerIDs[0]

	t.Logf("breaking and reverting machine ID: %s", machineID)

	// Mirrors StuckBootingKernelArg in talemu's internal/pkg/constants.
	breakingArg := "talemu.stuck=booting"

	if !clusterUsesEmulatedMachines(ctx, t, omniState, clusterName) {
		breakingArg = "maxcpus=2"

		// The kubelet patch must be fully applied and kubelet healthy again BEFORE the kernel args
		// change: the pending kernel args upgrade would otherwise block the config apply, and the
		// breakage would never arm.
		t.Run("KubeletReservedCPUsPatchShouldBeApplied", func(t *testing.T) {
			patch := omni.NewConfigPatch("500-" + clusterName + "-kubelet-reserved-cpus")

			createOrUpdate(ctx, t, omniState, patch, func(cps *omni.ConfigPatch) error {
				cps.Metadata().Labels().Set(omni.LabelCluster, clusterName)
				cps.Metadata().Labels().Set(omni.LabelClusterMachine, machineID)

				return cps.TypedSpec().Value.SetUncompressedData([]byte(kubeletReservedCPUsPatch))
			})

			// The patch is provably part of the applied config, not merely created: asserting only
			// on ConfigUpToDate could pass before the patch propagates into the desired config.
			rtestutils.AssertResource(ctx, t, omniState, machineID, func(res *omni.ClusterMachineConfigStatus, assert *assert.Assertions) {
				data, err := res.TypedSpec().Value.GetUncompressedData()
				if !assert.NoError(err) {
					return
				}

				defer data.Free()

				assert.Contains(string(data.Data()), "reserved-cpus")
			})

			rtestutils.AssertResource(ctx, t, omniState, machineID, func(res *omni.ClusterMachineStatus, assert *assert.Assertions) {
				assert.True(res.TypedSpec().Value.ConfigUpToDate, resourceDetails(res))
				assert.True(res.TypedSpec().Value.Ready, resourceDetails(res))
			})
		})
	}

	t.Run("BreakingKernelArgsShouldMakeMachineUnhealthy", func(t *testing.T) {
		_, err := safe.StateModifyWithResult(ctx, omniState, omni.NewKernelArgs(machineID), func(res *omni.KernelArgs) error {
			res.TypedSpec().Value.Args = []string{breakingArg}

			return nil
		})
		require.NoError(t, err)

		// The machine reboots into the new schematic and comes back broken: connected but not ready.
		rtestutils.AssertResource(ctx, t, omniState, machineID, func(res *omni.MachineStatus, assert *assert.Assertions) {
			assert.Contains(res.TypedSpec().Value.KernelCmdline, breakingArg, resourceDetails(res))
		})

		rtestutils.AssertResource(ctx, t, omniState, machineID, func(res *omni.ClusterMachineStatus, assert *assert.Assertions) {
			assert.False(res.TypedSpec().Value.Ready, resourceDetails(res))
		})

		clusterMachineStatus, err := safe.StateGetByID[*omni.ClusterMachineStatus](ctx, omniState, machineID)
		require.NoError(t, err)

		// The stage the breakage lands in is informational: booting and running-but-not-ready are both
		// covered by the fix, and a real kubelet crashloop can present as either.
		t.Logf("broken machine stage: %s", clusterMachineStatus.TypedSpec().Value.Stage)
	})

	t.Run("RevertedKernelArgsShouldRecoverMachine", func(t *testing.T) {
		_, err := safe.StateModifyWithResult(ctx, omniState, omni.NewKernelArgs(machineID), func(res *omni.KernelArgs) error {
			res.TypedSpec().Value.Args = nil

			return nil
		})
		require.NoError(t, err)

		// Before the fix this hangs forever: the machine is not healthy, so Omni never applies the
		// reverted args to it.
		rtestutils.AssertResource(ctx, t, omniState, machineID, func(res *omni.MachineStatus, assert *assert.Assertions) {
			assert.NotContains(res.TypedSpec().Value.KernelCmdline, breakingArg, resourceDetails(res))
		})

		rtestutils.AssertResource(ctx, t, omniState, machineID, func(res *omni.ClusterMachineStatus, assert *assert.Assertions) {
			assert.True(res.TypedSpec().Value.Ready, resourceDetails(res))
		})
	})

	t.Run("ClusterShouldBeDestroyed", AssertDestroyCluster(t.Context(), options.omniClient.Omni().State(), clusterName, false, false))
}
