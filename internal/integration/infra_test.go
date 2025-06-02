// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build integration

package integration_test

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

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
)

// machineProvisionHook creates a machine request set and waits until all requests are fulfilled.
//
//nolint:gocognit
func machineProvisionHook(t *testing.T, client *client.Client, cfg MachineProvisionConfig, machineRequestSetName,
	talosVersion string,
) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute*5)
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

// machineDeprovisionHook removes the machine request set and checks that all related links were deleted.
func machineDeprovisionHook(t *testing.T, client *client.Client, machineRequestSetName string) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute*5)
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

// infraMachinesAcceptHook asserts that there are a certain number of machines that are not accepted, provisioned by the static infra provider with the given ID.
//
// It then accepts them all and asserts that the states of various resources are updated as expected.
func infraMachinesAcceptHook(t *testing.T, omniState state.State, infraProviderID string, expectedCount int, disableKexec bool) {
	const disableKexecConfigPatch = `machine:
  install:
    extraKernelArgs:
      - kexec_load_disabled=1
  sysctls:
    kernel.kexec_load_disabled: "1"`

	logger := zaptest.NewLogger(t)

	ctx, cancel := context.WithTimeout(t.Context(), time.Minute*10)
	defer cancel()

	linksMap := make(map[string]*siderolink.Link, expectedCount)

	err := retry.Constant(time.Minute*10).RetryWithContext(ctx, func(ctx context.Context) error {
		links, err := safe.ReaderListAll[*siderolink.Link](ctx, omniState)
		if err != nil {
			return err
		}

		discoveredLinks := 0

		for link := range links.All() {
			providerID, ok := link.Metadata().Annotations().Get(omni.LabelInfraProviderID)
			if !ok {
				continue
			}

			if infraProviderID == providerID {
				discoveredLinks++
			}

			linksMap[link.Metadata().ID()] = link
		}

		if discoveredLinks != expectedCount {
			return retry.ExpectedErrorf("expected %d static infra provider machines, got %d", expectedCount, discoveredLinks)
		}

		return nil
	})

	require.NoError(t, err)

	// link count should match the expected count
	require.Equal(t, expectedCount, len(linksMap))

	ids := make([]resource.ID, 0, len(linksMap))

	for id := range linksMap {
		ids = append(ids, id)

		rtestutils.AssertResource(ctx, t, omniState, id, func(res *infra.Machine, assertion *assert.Assertions) {
			assertion.Equal(specs.InfraMachineConfigSpec_PENDING, res.TypedSpec().Value.AcceptanceStatus)
		})

		rtestutils.AssertNoResource[*infra.MachineStatus](ctx, t, omniState, id)

		rtestutils.AssertNoResource[*omni.Machine](ctx, t, omniState, id)

		// Accept the machine
		infraMachineConfig := omni.NewInfraMachineConfig(resources.DefaultNamespace, id)

		infraMachineConfig.TypedSpec().Value.AcceptanceStatus = specs.InfraMachineConfigSpec_ACCEPTED

		if disableKexec {
			infraMachineConfig.TypedSpec().Value.ExtraKernelArgs = "kexec_load_disabled=1"
		}

		require.NoError(t, omniState.Create(ctx, infraMachineConfig))

		if disableKexec {
			disableKexecConfigPatchRes := omni.NewConfigPatch(resources.DefaultNamespace, fmt.Sprintf("500-%s-disable-kexec", id))

			disableKexecConfigPatchRes.Metadata().Labels().Set(omni.LabelMachine, id)

			require.NoError(t, disableKexecConfigPatchRes.TypedSpec().Value.SetUncompressedData([]byte(disableKexecConfigPatch)))
			require.NoError(t, omniState.Create(ctx, disableKexecConfigPatchRes))
		}
	}

	logger.Info("accepted machines", zap.Reflect("infra_provider_id", infraProviderID), zap.Strings("machine_ids", ids))

	// Assert that the infra.Machines are now marked as accepted
	rtestutils.AssertResources(ctx, t, omniState, ids, func(res *infra.Machine, assertion *assert.Assertions) {
		assertion.Equal(specs.InfraMachineConfigSpec_ACCEPTED, res.TypedSpec().Value.AcceptanceStatus)
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

	// Assert that infra.MachineStatus resources are now created, powered off, marked as ready to use, and the machine labels are set on them
	rtestutils.AssertResources(ctx, t, omniState, ids, func(res *infra.MachineStatus, assertion *assert.Assertions) {
		aVal, _ := res.Metadata().Labels().Get("a")
		assertion.Equal("b", aVal)

		_, cOk := res.Metadata().Labels().Get("c")
		assertion.True(cOk)

		assertion.Equal(specs.InfraMachineStatusSpec_POWER_STATE_OFF, res.TypedSpec().Value.PowerState)
		assertion.True(res.TypedSpec().Value.ReadyToUse)
	})

	// Assert the infra provider labels on MachineStatus resources
	rtestutils.AssertResources(ctx, t, omniState, ids, func(res *omni.MachineStatus, assertion *assert.Assertions) {
		link := linksMap[res.Metadata().ID()]

		infraProviderID, _ := link.Metadata().Annotations().Get(omni.LabelInfraProviderID)

		aLabel := fmt.Sprintf(omni.InfraProviderLabelPrefixFormat, infraProviderID) + "a"
		aVal, _ := res.Metadata().Labels().Get(aLabel)

		assertion.Equal("b", aVal)

		cLabel := fmt.Sprintf(omni.InfraProviderLabelPrefixFormat, infraProviderID) + "c"
		_, cOk := res.Metadata().Labels().Get(cLabel)
		assertion.True(cOk)
	})
}

// infraMachinesDestroyHook removes siderolink.Link resources for all machines managed by a static infra provider,
// and asserts that the related infra.Machine and infra.MachineStatus resources are deleted.
func infraMachinesDestroyHook(t *testing.T, omniState state.State, providerID string, count int) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute*10)
	defer cancel()

	links, err := safe.StateListAll[*siderolink.Link](ctx, omniState)
	require.NoError(t, err)

	var deleted int

	for link := range links.All() {
		pid, ok := link.Metadata().Annotations().Get(omni.LabelInfraProviderID)
		if !ok {
			continue
		}

		if pid != providerID {
			continue
		}

		id := link.Metadata().ID()

		rtestutils.Destroy[*siderolink.Link](ctx, t, omniState, []string{id})

		rtestutils.AssertNoResource[*infra.Machine](ctx, t, omniState, id)
		rtestutils.AssertNoResource[*infra.MachineStatus](ctx, t, omniState, id)

		deleted++
	}

	require.EqualValues(t, count, deleted)
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

			// There must be an infra.Machine resource for each node
			rtestutils.AssertResource[*infra.Machine](ctx, t, omniState, id, func(res *infra.Machine, assertion *assert.Assertions) {
				assertion.Equal(talosVersion, res.TypedSpec().Value.ClusterTalosVersion)
				assertion.Empty(res.TypedSpec().Value.WipeId)
				assertion.Equal(extensions, res.TypedSpec().Value.Extensions)
			})

			// The machine is allocated, so it will be powered on and be ready to use
			rtestutils.AssertResource[*infra.MachineStatus](ctx, t, omniState, id, func(res *infra.MachineStatus, assertion *assert.Assertions) {
				assertion.Equal(specs.InfraMachineStatusSpec_POWER_STATE_ON, res.TypedSpec().Value.PowerState)
				assertion.True(res.TypedSpec().Value.ReadyToUse)
				assertion.True(res.TypedSpec().Value.Installed)
			})
		}
	}
}
