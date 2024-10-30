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
