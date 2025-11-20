// Copyright (c) 2025 Sidero Labs, Inc.
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

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

func testKernelArgsUpdate(t *testing.T, options *TestOptions) {
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Minute)
	defer cancel()

	clusterName := "integration-kernel-args-update"

	// Create a cluster to make sure that we have Talos installed on a machine
	t.Run(
		"ClusterShouldBeCreated",
		CreateCluster(t.Context(), options.omniClient, ClusterOptions{
			Name:          clusterName,
			ControlPlanes: 1,

			MachineOptions: options.MachineOptions,
			ScalingTimeout: options.ScalingTimeout,

			SkipExtensionCheckOnCreate: options.SkipExtensionsCheckOnCreate,

			// Pick machines which are booted with UKI, as kernel args upgrades are only supported for them.
			PickFilterFunc: func(ms *omni.MachineStatus, _ []*omni.MachineStatus) bool {
				return ms.TypedSpec().Value.GetSecurityState().GetBootedWithUki()
			},
		}),
	)

	assertClusterAndAPIReady(t, clusterName, options)

	omniState := options.omniClient.Omni().State()

	var machineID resource.ID

	t.Run("ClusterMachineKernelArgsShouldBeUpdated", func(t *testing.T) {
		clusterMachines, err := safe.StateListAll[*omni.ClusterMachine](ctx, omniState, state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, clusterName)))
		require.NoError(t, err)

		machineID = clusterMachines.Get(0).Metadata().ID()

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

		// verify that the new kernel args appear in kernel cmdline
		rtestutils.AssertResource(ctx, t, omniState, machineID, func(r *omni.MachineStatus, assert *assert.Assertions) {
			assert.Contains(r.TypedSpec().Value.KernelCmdline, strings.Join(kernelArgs.TypedSpec().Value.Args, " "), resourceDetails(r))
		})
	})

	t.Run("ClusterShouldBeDestroyed", AssertDestroyCluster(t.Context(), options.omniClient.Omni().State(), clusterName, false, false))

	t.Run("MachineKernelArgsShouldBeUpdatedInMaintenance", func(t *testing.T) {
		kernelArgs, err := safe.StateModifyWithResult(ctx, omniState, omni.NewKernelArgs(machineID), func(res *omni.KernelArgs) error {
			res.TypedSpec().Value.Args = []string{"maintenance=true"}

			return nil
		})
		require.NoError(t, err)

		// verify that the new kernel args appear in kernel cmdline
		rtestutils.AssertResource(ctx, t, omniState, machineID, func(r *omni.MachineStatus, assert *assert.Assertions) {
			assert.Contains(r.TypedSpec().Value.KernelCmdline, strings.Join(kernelArgs.TypedSpec().Value.Args, " "), resourceDetails(r))
		})
	})
}
