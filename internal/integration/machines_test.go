// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build integration

package integration_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"github.com/siderolabs/omni/client/pkg/client/management"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
)

// AssertMachineStatus verifies that all machines have their MachineStatus populated.
func AssertMachineStatus(testCtx context.Context, st state.State, shouldMaintenance bool, clusterName string, expectedLabels map[string]string, deniedLabels []string) TestFunc {
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(testCtx, 120*time.Second)
		defer cancel()

		var (
			opts []state.ListOption
			ids  = rtestutils.ResourceIDs[*omni.Machine](ctx, t, st)
		)

		if clusterName != "" {
			opts = append(opts, state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, clusterName)))
			ids = rtestutils.ResourceIDs[*omni.MachineSetNode](ctx, t, st, opts...)
		}

		rtestutils.AssertResources(ctx, t, st,
			ids,
			func(machineStatus *omni.MachineStatus, assert *assert.Assertions) {
				spec := machineStatus.TypedSpec().Value

				assert.NotEmpty(spec.ManagementAddress, resourceDetails(machineStatus))
				assert.True(spec.Connected, resourceDetails(machineStatus))
				assert.Equal(shouldMaintenance, spec.Maintenance, resourceDetails(machineStatus))
				assert.Empty(spec.LastError, resourceDetails(machineStatus))

				assert.NotEmpty(spec.TalosVersion, resourceDetails(machineStatus))

				assert.NotEmpty(spec.GetNetwork().GetHostname(), resourceDetails(machineStatus))
				assert.NotEmpty(spec.GetNetwork().GetAddresses(), resourceDetails(machineStatus))
				assert.NotEmpty(spec.GetNetwork().GetDefaultGateways(), resourceDetails(machineStatus))
				assert.NotEmpty(spec.GetNetwork().GetNetworkLinks(), resourceDetails(machineStatus))

				assert.NotEmpty(spec.GetHardware().GetBlockdevices(), resourceDetails(machineStatus))
				assert.NotEmpty(spec.GetHardware().GetMemoryModules(), resourceDetails(machineStatus))
				assert.NotEmpty(spec.GetHardware().GetProcessors(), resourceDetails(machineStatus))

				for k, v := range expectedLabels {
					lv, ok := machineStatus.Metadata().Labels().Get(k)
					assert.True(ok, "label %q is not set: %s", k, resourceDetails(machineStatus))
					assert.Equal(v, lv, "label %q has unexpected value: %q != %q: %s", k, v, lv, resourceDetails(machineStatus))
				}

				for _, k := range deniedLabels {
					_, ok := machineStatus.Metadata().Labels().Get(k)
					assert.False(ok, "label %q is set: %s", k, resourceDetails(machineStatus))
				}
			}, rtestutils.WithReportInterval(time.Second*5))
	}
}

// AssertMachinesHaveLogs verifies that all machines have their kernel logs coming.
func AssertMachinesHaveLogs(testCtx context.Context, st state.State, managementClient *management.Client) TestFunc {
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(testCtx, 20*time.Second)
		defer cancel()

		machineIDs := rtestutils.ResourceIDs[*omni.Machine](ctx, t, st)

		eg, ctx := errgroup.WithContext(ctx)

		for _, machineID := range machineIDs {
			eg.Go(func() error {
				return retry.Constant(time.Second*20, retry.WithUnits(time.Second)).RetryWithContext(ctx, func(ctx context.Context) error {
					logR, err := managementClient.LogsReader(ctx, machineID, true, -1)
					if err != nil {
						return retry.ExpectedError(err)
					}

					bufR := bufio.NewReader(logR)

					line, err := bufR.ReadBytes('\n')
					if err != nil {
						return retry.ExpectedError(err)
					}

					line = bytes.TrimSpace(line)

					if len(line) == 0 {
						return retry.ExpectedErrorf("empty log line")
					}

					var msg map[string]any

					return json.Unmarshal(line, &msg)
				})
			})
		}

		assert.NoError(t, eg.Wait())
	}
}

// AssertUnallocatedMachineDestroyFlow destroys a siderolink.Link resource and verifies that unallocated Machine is removed.
//
// Once the Machine is removed, it reboots the VM and asserts that machine re-registers itself.
func AssertUnallocatedMachineDestroyFlow(testCtx context.Context, options *TestOptions, st state.State, restartAMachineFunc RestartAMachineFunc) TestFunc {
	return func(t *testing.T) {
		if restartAMachineFunc == nil {
			t.Skip("restartAMachineFunc is nil")
		}

		ctx, cancel := context.WithTimeout(testCtx, 90*time.Second)
		defer cancel()

		require := require.New(t)

		pickUnallocatedMachines(ctx, t, st, 1, nil, func(machineIDs []string) {
			rtestutils.Destroy[*siderolink.Link](ctx, t, st, machineIDs)

			rtestutils.AssertNoResource[*omni.Machine](ctx, t, st, machineIDs[0])
			rtestutils.AssertNoResource[*omni.MachineStatus](ctx, t, st, machineIDs[0])

			// reboot a machine
			require.NoError(restartAMachineFunc(ctx, machineIDs[0]))

			// machine should re-register and become available
			rtestutils.AssertResources(ctx, t, st, machineIDs, func(machine *omni.MachineStatus, assert *assert.Assertions) {
				_, ok := machine.Metadata().Labels().Get(omni.MachineStatusLabelAvailable)
				assert.True(ok)
			})
		})
	}
}

// AssertForceRemoveWorkerNode destroys a Link for the worker node and verifies that allocated Machine is removed as part of a cluster.
//
// The VM is wiped & rebooted to bring it back as an available machine.
func AssertForceRemoveWorkerNode(testCtx context.Context, st state.State, clusterName string, freezeAMachineFunc FreezeAMachineFunc, wipeAMachineFunc WipeAMachineFunc) TestFunc {
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(testCtx, 90*time.Second)
		defer cancel()

		if wipeAMachineFunc == nil {
			t.Skip("wipeAMachineFunc is nil")
		}

		if freezeAMachineFunc == nil {
			t.Skip("freezeAMachineFunc is nil")
		}

		id := freezeMachine(ctx, t, st, clusterName, freezeAMachineFunc, omni.LabelWorkerRole)

		wipeMachine(ctx, t, st, id, wipeAMachineFunc)
	}
}

// AssertControlPlaneForceReplaceMachine freezes a control plane machine, scales controlplane by one, destroys and wipes frozen machine.
//
// If the controlplane had 3 machines, and one is frozen, the fourth machine can't be added until a etcd member
// is removed.
// The VM is wiped & rebooted to bring it back as an available machine.
func AssertControlPlaneForceReplaceMachine(testCtx context.Context, st state.State, clusterName string, options Options) TestFunc {
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(testCtx, 5*time.Minute)
		defer cancel()

		if options.WipeAMachineFunc == nil {
			t.Skip("wipeAMachineFunc is nil")
		}

		if options.FreezeAMachineFunc == nil {
			t.Skip("freezeAMachineFunc is nil")
		}

		id := freezeMachine(ctx, t, st, clusterName, options.FreezeAMachineFunc, omni.LabelControlPlaneRole)

		pickUnallocatedMachines(ctx, t, st, 1, nil, func(machineIDs []resource.ID) {
			t.Logf("Adding machine '%s' to control plane", machineIDs[0])

			bindMachine(ctx, t, st, bindMachineOptions{
				clusterName:  clusterName,
				role:         omni.LabelControlPlaneRole,
				machineID:    machineIDs[0],
				talosVersion: options.MachineOptions.TalosVersion,
			})

			// assert that machines got allocated (label available is removed)
			rtestutils.AssertResources(ctx, t, st, machineIDs, func(machineStatus *omni.MachineStatus, assert *assert.Assertions) {
				assert.True(machineStatus.Metadata().Labels().Matches(
					resource.LabelTerm{
						Key:    omni.MachineStatusLabelAvailable,
						Op:     resource.LabelOpExists,
						Invert: true,
					},
				), resourceDetails(machineStatus))
			})
		})

		wipeMachine(ctx, t, st, id, options.WipeAMachineFunc)
	}
}

// freezeMachinesOfType freezes all machines of a given type.
func freezeMachinesOfType(ctx context.Context, t *testing.T, st state.State, clusterName string, freezeAMachineFunc FreezeAMachineFunc, machineType string) []string {
	machineIDSet := rtestutils.ResourceIDs[*omni.MachineSetNode](ctx, t, st, state.WithLabelQuery(
		resource.LabelEqual(omni.LabelCluster, clusterName),
		resource.LabelExists(machineType),
	))
	require.NotEmpty(t, machineIDSet)

	for _, machineID := range machineIDSet {
		require.NoError(t, freezeAMachineFunc(ctx, machineID))
	}

	return machineIDSet
}

// freezeMachine freezes the VM CPU, which simulates a hardware failure.
func freezeMachine(ctx context.Context, t *testing.T, st state.State, clusterName string, freezeAMachineFunc FreezeAMachineFunc, machineType string) string {
	require := require.New(t)

	machineIDs := rtestutils.ResourceIDs[*omni.MachineSetNode](ctx, t, st, state.WithLabelQuery(
		resource.LabelEqual(omni.LabelCluster, clusterName),
		resource.LabelExists(machineType),
	))

	require.NotEmpty(machineIDs)

	// freeze a machine to simulate hardware failure
	require.NoError(freezeAMachineFunc(ctx, machineIDs[0]))

	return machineIDs[0]
}

func wipeMachine(ctx context.Context, t *testing.T, st state.State, id string, wipeAMachineFunc WipeAMachineFunc) {
	// force delete a machine
	rtestutils.Teardown[*siderolink.Link](ctx, t, st, []string{id})
	rtestutils.Destroy[*omni.MachineSetNode](ctx, t, st, []string{id})
	rtestutils.Destroy[*siderolink.Link](ctx, t, st, []string{id})

	// now machine should be removed
	rtestutils.AssertNoResource[*omni.Machine](ctx, t, st, id)
	rtestutils.AssertNoResource[*omni.ClusterMachine](ctx, t, st, id)

	// wipe and reboot a machine
	require.NoError(t, wipeAMachineFunc(ctx, id))

	// machine should re-register
	rtestutils.AssertResources(ctx, t, st, []string{id}, func(*omni.Machine, *assert.Assertions) {})

	t.Logf("Wiped the machine '%s'", id)
}
