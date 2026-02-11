// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build integration

package integration_test

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/blang/semver/v4"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	xmaps "github.com/siderolabs/gen/maps"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

func testKernelArgsUpdate(t *testing.T, options *TestOptions) {
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Minute)
	defer cancel()

	options.claimMachines(t, 2)

	clusterName := "integration-kernel-args-update"

	version, err := semver.ParseTolerant(options.MachineOptions.TalosVersion)
	require.NoError(t, err)

	// Create a cluster to make sure that we have Talos installed on a machine
	t.Run(
		"ClusterShouldBeCreated",
		CreateCluster(t.Context(), options, ClusterOptions{
			Name:          clusterName,
			ControlPlanes: 1,
			Workers:       1,

			MachineOptions: options.MachineOptions,
			ScalingTimeout: options.ScalingTimeout,

			SkipExtensionCheckOnCreate: options.SkipExtensionsCheckOnCreate,

			// Pick two machines depending on the Talos version:
			// - 1.12 or later; one booted with UKI and another booted with non-UKI, as kernel args upgrades are supported with Talos 1.12 and later.
			// - earlier versions; pick machines booted with UKI, because they are the only ones that support kernel args upgrades.
			PickFilterFunc: func(ms *omni.MachineStatus, alreadyPicked []*omni.MachineStatus) bool {
				canUseUKICmdline := version.Major > 1 || (version.Major == 1 && version.Minor >= 12)

				// If we can't let grub to use UKI cmdline, we can only pick machines with UKI.
				if !canUseUKICmdline {
					return ms.TypedSpec().Value.GetSecurityState().GetBootedWithUki()
				}

				alreadyPickedUKI := slices.ContainsFunc(alreadyPicked, func(m *omni.MachineStatus) bool { return m.TypedSpec().Value.GetSecurityState().GetBootedWithUki() })
				alreadyPickedNonUKI := slices.ContainsFunc(alreadyPicked, func(m *omni.MachineStatus) bool { return !m.TypedSpec().Value.GetSecurityState().GetBootedWithUki() })

				if alreadyPickedUKI && alreadyPickedNonUKI {
					return true
				}

				isUKI := ms.TypedSpec().Value.GetSecurityState().GetBootedWithUki()
				if isUKI && !alreadyPickedUKI {
					return true
				}

				return !isUKI && !alreadyPickedNonUKI
			},
		}),
	)

	assertClusterAndAPIReady(t, clusterName, options)

	omniState := options.omniClient.Omni().State()

	kernelArgsMap := make(map[string]*omni.KernelArgs, 2)

	t.Run("ClusterMachineKernelArgsShouldBeUpdated", func(t *testing.T) {
		clusterMachines, err := safe.StateListAll[*omni.ClusterMachine](ctx, omniState, state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, clusterName)))
		require.NoError(t, err)

		for machine := range clusterMachines.All() {
			machineID := machine.Metadata().ID()

			t.Logf("picked machine ID: %s", machineID)

			kernelArgs, err := safe.StateModifyWithResult(ctx, omniState, omni.NewKernelArgs(machineID), func(res *omni.KernelArgs) error {
				// make sure that we actually change the args by checking the existing value
				args1 := []string{"foo=bar", "baz=qux"}
				args2 := []string{"baz=qux", "foo=bar"}

				if slices.Equal(res.TypedSpec().Value.Args, args1) {
					res.TypedSpec().Value.Args = args2

					return nil
				}

				res.TypedSpec().Value.Args = args1

				return nil
			})
			require.NoError(t, err)

			kernelArgsMap[machineID] = kernelArgs
		}

		// verify that the new kernel args appear in the kernel cmdline
		rtestutils.AssertResources(ctx, t, omniState, xmaps.Keys(kernelArgsMap), func(r *omni.MachineStatus, assert *assert.Assertions) {
			assert.Contains(r.TypedSpec().Value.KernelCmdline, strings.Join(kernelArgsMap[r.Metadata().ID()].TypedSpec().Value.Args, " "), resourceDetails(r))
		})
	})

	t.Run("ClusterShouldBeDestroyed", AssertDestroyCluster(t.Context(), options.omniClient.Omni().State(), clusterName, false, false))

	updatedKernelArgsMap := make(map[string]*omni.KernelArgs, 2)

	t.Run("MachineKernelArgsShouldBeUpdatedInMaintenance", func(t *testing.T) {
		for machineID := range kernelArgsMap {
			updatedKernelArgs, err := safe.StateModifyWithResult(ctx, omniState, omni.NewKernelArgs(machineID), func(res *omni.KernelArgs) error {
				res.TypedSpec().Value.Args = []string{"maintenance=true"}

				return nil
			})
			require.NoError(t, err)

			updatedKernelArgsMap[machineID] = updatedKernelArgs
		}

		// verify that the new kernel args appear in the kernel cmdline
		rtestutils.AssertResources(ctx, t, omniState, xmaps.Keys(updatedKernelArgsMap), func(r *omni.MachineStatus, assert *assert.Assertions) {
			assert.Contains(r.TypedSpec().Value.KernelCmdline, strings.Join(updatedKernelArgsMap[r.Metadata().ID()].TypedSpec().Value.Args, " "), resourceDetails(r))
		})
	})
}
