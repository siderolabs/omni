// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build integration

package integration_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/pair"
	talosclient "github.com/siderolabs/talos/pkg/machinery/client"
	talosruntime "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

const (
	// these markers are baked into the embedded machine configuration in hack/test/templates/embedded-config-schematic.yaml.
	embeddedConfigEnvMarker  = "omni-embedded-config-env-marker"
	embeddedConfigHostMarker = "omni-embedded-config-host-marker"

	// initialMachineConfigPatchPrefix is the ID prefix of the config patch Omni extracts from a machine's incoming configuration.
	initialMachineConfigPatchPrefix = "000-initial-machine-config-"

	// maintenanceApplyMarker is the environment variable value the test applies to a maintenance machine to observe the apply.
	maintenanceApplyMarker = "omni-maintenance-apply-marker"
)

func testEmbeddedMachineConfig(options *TestOptions) TestFunc {
	return func(t *testing.T) {
		t.Log(`
Test the lifecycle of a machine that arrives at Omni carrying its own embedded configuration.

Step 1: machines booted from an image with documents baked into embeddedMachineConfiguration should have their
non-connection documents captured into a user-owned config patch, while the SideroLink connection documents are dropped.
Step 2: a machine config patch applied to a machine while it is in maintenance mode should reach the machine, observed
through the environment variable it sets propagating to the machine's Environment resource.`)

		t.Parallel()

		options.claimMachines(t, 1)

		t.Run(
			"EmbeddedConfigShouldBeExtracted",
			AssertEmbeddedConfigExtracted(t.Context(), options.omniClient.Omni().State()),
		)

		t.Run(
			"ConfigPatchShouldApplyInMaintenance",
			AssertMachineConfigPatchAppliedInMaintenance(t.Context(), options),
		)
	}
}

// AssertEmbeddedConfigExtracted asserts that machines which connected carrying an embedded machine configuration had
// their non-connection documents captured into a user-owned config patch, while the SideroLink connection documents
// were dropped.
func AssertEmbeddedConfigExtracted(testCtx context.Context, omniState state.State) TestFunc {
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(testCtx, 5*time.Minute)
		defer cancel()

		machineStatusList, err := safe.StateListAll[*omni.MachineStatus](ctx, omniState)
		require.NoError(t, err)

		var embeddedMachineIDs []resource.ID

		// a machine that arrived with an embedded configuration carries it in its raw schematic
		for machineStatus := range machineStatusList.All() {
			if strings.Contains(machineStatus.TypedSpec().Value.GetSchematic().GetRaw(), embeddedConfigEnvMarker) {
				embeddedMachineIDs = append(embeddedMachineIDs, machineStatus.Metadata().ID())
			}
		}

		require.NotEmpty(t, embeddedMachineIDs, "no machines with an embedded machine configuration found")

		t.Logf("found machines with an embedded machine configuration: %v", embeddedMachineIDs)

		for _, machineID := range embeddedMachineIDs {
			rtestutils.AssertResource[*omni.ConfigPatch](ctx, t, omniState, initialMachineConfigPatchPrefix+machineID,
				func(patch *omni.ConfigPatch, assertion *assert.Assertions) {
					buffer, bufErr := patch.TypedSpec().Value.GetUncompressedData()
					assertion.NoError(bufErr)

					defer buffer.Free()

					data := string(buffer.Data())

					// the user documents are preserved, with their markers intact
					assertion.Contains(data, embeddedConfigEnvMarker)
					assertion.Contains(data, embeddedConfigHostMarker)

					// the Omni-managed connection documents are dropped
					assertion.NotContains(data, "SideroLinkConfig")
					assertion.NotContains(data, "EventSinkConfig")
					assertion.NotContains(data, "KmsgLogConfig")
				})
		}
	}
}

// AssertMachineConfigPatchAppliedInMaintenance asserts that a machine-level config patch is applied to a machine while
// it is in maintenance mode. It applies an EnvironmentConfig document and observes the environment variable propagate to
// the machine's Environment resource, which only happens once the config is applied.
func AssertMachineConfigPatchAppliedInMaintenance(testCtx context.Context, options *TestOptions) TestFunc {
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(testCtx, 10*time.Minute)
		defer cancel()

		omniState := options.omniClient.Omni().State()

		inMaintenance := func(machineStatus *omni.MachineStatus, _ []*omni.MachineStatus) bool {
			return machineStatus.TypedSpec().Value.Maintenance
		}

		var machineID resource.ID

		pickUnallocatedMachines(ctx, t, omniState, 1, inMaintenance, func(machineIDs []resource.ID) {
			machineID = machineIDs[0]
		})

		t.Logf("applying a config patch to maintenance machine %q", machineID)

		patch := omni.NewConfigPatch("999-maintenance-apply-test-"+machineID, pair.MakePair(omni.LabelMachine, machineID))

		require.NoError(t, safe.StateModify(ctx, omniState, patch, func(p *omni.ConfigPatch) error {
			return p.TypedSpec().Value.SetUncompressedData([]byte("apiVersion: v1alpha1\n" +
				"kind: EnvironmentConfig\n" +
				"variables:\n" +
				"    OMNI_MAINTENANCE_APPLY_MARKER: " + maintenanceApplyMarker + "\n"))
		}))

		t.Cleanup(func() {
			rtestutils.Destroy[*omni.ConfigPatch](testCtx, t, omniState, []string{patch.Metadata().ID()})
		})

		talosClient := getTalosClient(ctx, t, options)

		t.Cleanup(func() {
			require.NoError(t, talosClient.Close())
		})

		nodeCtx := talosclient.WithNode(ctx, machineID)

		// the env var lands in the machine's Environment resource (fixed ID "machined") once Omni applies the patch in maintenance mode
		rtestutils.AssertResource[*talosruntime.Environment](nodeCtx, t, talosClient.COSI, "machined",
			func(environment *talosruntime.Environment, assertion *assert.Assertions) {
				assertion.Contains(strings.Join(environment.TypedSpec().Variables, " "), maintenanceApplyMarker,
					"env var not propagated to maintenance machine %q", machineID)
			})
	}
}
