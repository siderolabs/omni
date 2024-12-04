// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package tests

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
)

// AssertMachinesShouldBeProvisioned creates a machine request set and waits until all requests are fulfilled.
//
//nolint:gocognit
func AssertMachinesShouldBeProvisioned(testCtx context.Context, client *client.Client, cfg MachineProvisionConfig, machineRequestSetName,
	talosVersion string,
) TestFunc {
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(testCtx, time.Minute*5)
		defer cancel()

		rtestutils.AssertResources(ctx, t, client.Omni().State(), []string{cfg.Provider.ID}, func(*infra.ProviderStatus, *assert.Assertions) {})

		machineRequestSet, err := safe.ReaderGetByID[*omni.MachineRequestSet](ctx, client.Omni().State(), machineRequestSetName)

		if !state.IsNotFoundError(err) {
			require.NoError(t, err)
		}

		if machineRequestSet != nil {
			rtestutils.Destroy[*omni.MachineRequestSet](ctx, t, client.Omni().State(), []string{machineRequestSetName})
		}

		machineRequestSet = omni.NewMachineRequestSet(resources.DefaultNamespace, machineRequestSetName)

		machineRequestSet.TypedSpec().Value.Extensions = []string{
			"siderolabs/" + HelloWorldServiceExtensionName,
		}

		machineRequestSet.TypedSpec().Value.ProviderId = cfg.Provider.ID
		machineRequestSet.TypedSpec().Value.TalosVersion = talosVersion
		machineRequestSet.TypedSpec().Value.ProviderData = cfg.Provider.Data
		machineRequestSet.TypedSpec().Value.MachineCount = int32(cfg.MachineCount)

		require.NoError(t, client.Omni().State().Create(ctx, machineRequestSet))

		var resources safe.List[*infra.MachineRequestStatus]

		err = retry.Constant(time.Second*60).RetryWithContext(ctx, func(ctx context.Context) error {
			resources, err = safe.ReaderListAll[*infra.MachineRequestStatus](ctx, client.Omni().State(),
				state.WithLabelQuery(resource.LabelEqual(omni.LabelMachineRequestSet, machineRequestSetName)),
			)
			if err != nil {
				return err
			}

			if resources.Len() != cfg.MachineCount {
				return retry.ExpectedErrorf("provision machine count is %d, expected %d", resources.Len(), cfg.MachineCount)
			}

			return nil
		})

		require.NoError(t, err)

		err = retry.Constant(time.Minute*5).RetryWithContext(ctx, func(ctx context.Context) error {
			var machines safe.List[*omni.MachineStatus]

			machines, err = safe.ReaderListAll[*omni.MachineStatus](ctx, client.Omni().State())
			if err != nil {
				return err
			}

			if machines.Len() < cfg.MachineCount {
				return retry.ExpectedErrorf("links count is %d, expected at least %d", machines.Len(), cfg.MachineCount)
			}

			for r := range resources.All() {
				requestedMachines := machines.FilterLabelQuery(resource.LabelEqual(omni.LabelMachineRequest, r.Metadata().ID()))

				if requestedMachines.Len() == 0 {
					return retry.ExpectedErrorf("machine request %q doesn't have the related link", r.Metadata().ID())
				}

				if requestedMachines.Len() != 1 {
					return fmt.Errorf("more than one machine is labeled with %q machine request label", r.Metadata().ID())
				}

				m := requestedMachines.Get(0)
				if m.TypedSpec().Value.Hardware == nil {
					return retry.ExpectedErrorf("the machine %q is not fully provisioned", r.Metadata().ID())
				}
			}

			return nil
		})

		require.NoError(t, err)
	}
}

// AssertMachinesShouldBeDeprovisioned removes the machine request set and checks that all related links were deleted.
func AssertMachinesShouldBeDeprovisioned(testCtx context.Context, client *client.Client, machineRequestSetName string) TestFunc {
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(testCtx, time.Minute*5)
		defer cancel()

		requestIDs := rtestutils.ResourceIDs[*infra.MachineRequest](ctx, t, client.Omni().State(),
			state.WithLabelQuery(resource.LabelEqual(omni.LabelMachineRequestSet, machineRequestSetName)),
		)

		links, err := safe.ReaderListAll[*siderolink.Link](ctx, client.Omni().State())

		require.NoError(t, err)

		linkIDs := make([]string, 0, len(requestIDs))

		for l := range links.All() {
			mr, ok := l.Metadata().Labels().Get(omni.LabelMachineRequest)
			if !ok {
				continue
			}

			if slices.Index(requestIDs, mr) != -1 {
				linkIDs = append(linkIDs, l.Metadata().ID())
			}
		}

		rtestutils.Destroy[*omni.MachineRequestSet](ctx, t, client.Omni().State(), []string{machineRequestSetName})

		for _, id := range requestIDs {
			rtestutils.AssertNoResource[*infra.MachineRequest](ctx, t, client.Omni().State(), id)
		}

		for _, id := range linkIDs {
			rtestutils.AssertNoResource[*siderolink.Link](ctx, t, client.Omni().State(), id)
		}
	}
}

// AcceptInfraMachines asserts that there are a certain number of machines that are not accepted, provisioned by the static infra provider with the given ID.
//
// It then accepts them all and asserts that the states of various resources are updated as expected.
func AcceptInfraMachines(testCtx context.Context, omniState state.State, expectedCount int, disableKexec bool) TestFunc {
	const disableKexecConfigPatch = `machine:
  install:
    extraKernelArgs:
      - kexec_load_disabled=1
  sysctls:
    kernel.kexec_load_disabled: "1"`

	return func(t *testing.T) {
		logger := zaptest.NewLogger(t)

		ctx, cancel := context.WithTimeout(testCtx, time.Minute*10)
		defer cancel()

		rtestutils.AssertLength[*siderolink.Link](ctx, t, omniState, expectedCount)

		linkList, err := safe.StateListAll[*siderolink.Link](ctx, omniState)
		require.NoError(t, err)

		// link count should match the expected count
		require.Equal(t, expectedCount, linkList.Len())

		ids := make([]resource.ID, 0, linkList.Len())

		var infraProviderID string

		for link := range linkList.All() {
			ids = append(ids, link.Metadata().ID())

			infraProviderID, _ = link.Metadata().Annotations().Get(omni.LabelInfraProviderID)

			require.NotEmpty(t, infraProviderID)

			rtestutils.AssertResource[*infra.Machine](ctx, t, omniState, link.Metadata().ID(), func(res *infra.Machine, assertion *assert.Assertions) {
				assertion.False(res.TypedSpec().Value.Accepted)
			})

			rtestutils.AssertNoResource[*infra.MachineStatus](ctx, t, omniState, link.Metadata().ID())

			rtestutils.AssertNoResource[*omni.Machine](ctx, t, omniState, link.Metadata().ID())

			// Accept the machine
			infraMachineConfig := omni.NewInfraMachineConfig(resources.DefaultNamespace, link.Metadata().ID())

			infraMachineConfig.TypedSpec().Value.Accepted = true

			if disableKexec {
				infraMachineConfig.TypedSpec().Value.ExtraKernelArgs = "kexec_load_disabled=1"
			}

			require.NoError(t, omniState.Create(ctx, infraMachineConfig))

			if disableKexec {
				disableKexecConfigPatchRes := omni.NewConfigPatch(resources.DefaultNamespace, fmt.Sprintf("500-%s-disable-kexec", link.Metadata().ID()))

				disableKexecConfigPatchRes.Metadata().Labels().Set(omni.LabelMachine, link.Metadata().ID())

				require.NoError(t, disableKexecConfigPatchRes.TypedSpec().Value.SetUncompressedData([]byte(disableKexecConfigPatch)))
				require.NoError(t, omniState.Create(ctx, disableKexecConfigPatchRes))
			}
		}

		logger.Info("accepted machines", zap.String("infra_provider_id", infraProviderID), zap.Strings("machine_ids", ids))

		providerStatus, err := safe.StateGetByID[*infra.ProviderStatus](ctx, omniState, infraProviderID)
		require.NoError(t, err)

		_, isStaticProvider := providerStatus.Metadata().Labels().Get(omni.LabelIsStaticInfraProvider)
		require.True(t, isStaticProvider)

		// Assert that the infra.Machines are now marked as accepted
		rtestutils.AssertResources(ctx, t, omniState, ids, func(res *infra.Machine, assertion *assert.Assertions) {
			assertion.True(res.TypedSpec().Value.Accepted)
		})

		// Assert that omni.Machine resources are now created and marked as managed by the static infra provider
		rtestutils.AssertResources(ctx, t, omniState, ids, func(res *omni.Machine, assertion *assert.Assertions) {
			_, isManagedByStaticInfraProvider := res.Metadata().Labels().Get(omni.LabelIsManagedByStaticInfraProvider)

			assertion.True(isManagedByStaticInfraProvider)
		})

		// Assert that omni.Machine resources are now created
		rtestutils.AssertResources(ctx, t, omniState, ids, func(res *omni.Machine, assertion *assert.Assertions) {
			_, isManagedByStaticInfraProvider := res.Metadata().Labels().Get(omni.LabelIsManagedByStaticInfraProvider)

			assertion.True(isManagedByStaticInfraProvider)
		})

		// Assert that infra.MachineStatus resources are now created, and they are marked as ready to use
		rtestutils.AssertResources(ctx, t, omniState, ids, func(res *infra.MachineStatus, assertion *assert.Assertions) {
			assertion.True(res.TypedSpec().Value.ReadyToUse)
		})
	}
}

// AssertInfraMachinesAreAllocated asserts that the machines that belong to the given cluster and managed by a static infra provider
// are marked as allocated in the related resources.
func AssertInfraMachinesAreAllocated(testCtx context.Context, omniState state.State, clusterID, talosVersion string, extensions []string) TestFunc {
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(testCtx, time.Minute*10)
		defer cancel()

		nodeList, err := safe.StateListAll[*omni.MachineSetNode](ctx, omniState, state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, clusterID)))
		require.NoError(t, err)

		require.Greater(t, nodeList.Len(), 0)

		for machineSetNode := range nodeList.All() {
			id := machineSetNode.Metadata().ID()

			// there must be an infra.Machine resource for each node
			rtestutils.AssertResource[*infra.Machine](ctx, t, omniState, id, func(res *infra.Machine, assertion *assert.Assertions) {
				assertion.Equal(talosVersion, res.TypedSpec().Value.ClusterTalosVersion)
				assertion.Empty(res.TypedSpec().Value.WipeId)
				assertion.Equal(extensions, res.TypedSpec().Value.Extensions)
			})

			// machine is allocated, so the ReadyToUse field is set to false
			rtestutils.AssertResource[*infra.MachineStatus](ctx, t, omniState, id, func(res *infra.MachineStatus, assertion *assert.Assertions) {
				assertion.False(res.TypedSpec().Value.ReadyToUse)
			})

			// omni receives a SequenceEvent from the SideroLink event sink and sets the Installed field to true
			rtestutils.AssertResource[*infra.MachineState](ctx, t, omniState, id, func(res *infra.MachineState, assertion *assert.Assertions) {
				assertion.True(res.TypedSpec().Value.Installed)
			})
		}
	}
}

// AssertAllInfraMachinesAreUnallocated asserts that all infra machines are unallocated.
func AssertAllInfraMachinesAreUnallocated(testCtx context.Context, omniState state.State) TestFunc {
	return func(t *testing.T) {
		logger := zaptest.NewLogger(t)

		ctx, cancel := context.WithTimeout(testCtx, time.Minute*10)
		defer cancel()

		infraMachineList, err := safe.StateListAll[*infra.Machine](ctx, omniState)
		require.NoError(t, err)

		require.Greater(t, infraMachineList.Len(), 0)

		for infraMachine := range infraMachineList.All() {
			id := infraMachine.Metadata().ID()

			rtestutils.AssertResource[*infra.Machine](ctx, t, omniState, id, func(res *infra.Machine, assertion *assert.Assertions) {
				assertion.Empty(res.TypedSpec().Value.ClusterTalosVersion)
				assertion.Empty(res.TypedSpec().Value.Extensions)

				if assertion.NotEmpty(res.TypedSpec().Value.WipeId) { // the machine should be marked for wipe
					logger.Info("machine is marked for wipe", zap.String("machine_id", id), zap.String("wipe_id", res.TypedSpec().Value.WipeId))
				}
			})

			// machine is unallocated, so the ReadyToUse field will be set to true
			rtestutils.AssertResource[*infra.MachineStatus](ctx, t, omniState, id, func(res *infra.MachineStatus, assertion *assert.Assertions) {
				assertion.True(res.TypedSpec().Value.ReadyToUse)
			})

			// provider wipes the machine and sets the Installed field to false
			rtestutils.AssertResource[*infra.MachineState](ctx, t, omniState, id, func(res *infra.MachineState, assertion *assert.Assertions) {
				assertion.False(res.TypedSpec().Value.Installed)
			})
		}
	}
}

// DestroyInfraMachines removes siderolink.Link resources for all machines managed by a static infra provider,
// and asserts that the related infra.Machine and infra.MachineStatus resources are deleted.
func DestroyInfraMachines(testCtx context.Context, omniState state.State) TestFunc {
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(testCtx, time.Minute*10)
		defer cancel()

		machineList, err := safe.StateListAll[*omni.Machine](ctx, omniState, state.WithLabelQuery(resource.LabelExists(omni.LabelIsManagedByStaticInfraProvider)))
		require.NoError(t, err)

		require.Greater(t, machineList.Len(), 0)

		for machine := range machineList.All() {
			id := machine.Metadata().ID()

			rtestutils.Destroy[*siderolink.Link](ctx, t, omniState, []string{id})

			rtestutils.AssertNoResource[*infra.Machine](ctx, t, omniState, id)
			rtestutils.AssertNoResource[*infra.MachineStatus](ctx, t, omniState, id)
		}
	}
}
