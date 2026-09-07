// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package schematic_test

import (
	"context"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/image-factory/pkg/schematic"
	talosconstants "github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
	schematicctrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/schematic"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils"
)

// TestSchematicConfigurationFactoryRecord covers the factory recorded next to the schematic ID: the one that
// issued it, which an install image with that ID has to name. A factory switch makes the primary a factory
// that does not know the IDs issued before, so the record is what keeps the two together.
func TestSchematicConfigurationFactoryRecord(t *testing.T) {
	t.Parallel()

	const (
		primaryURL   = "https://primary.factory.test"
		secondaryURL = "https://secondary.factory.test"
		talosVersion = "1.10.0"
	)

	// The schematic the machine booted with, unchanged by the controller: the machine reports the same
	// extensions and kernel args, so the controller has no reason to go to the factory.
	raw := schematic.Schematic{
		Customization: schematic.Customization{
			ExtraKernelArgs: []string{"console=ttyS0"},
			SystemExtensions: schematic.SystemExtensions{
				OfficialExtensions: []string{"siderolabs/hello-world-service"},
			},
		},
	}

	rawYAML, err := raw.Marshal()
	require.NoError(t, err)

	rawID, err := raw.ID()
	require.NoError(t, err)

	newMachineStatus := func(name string) *omni.MachineStatus {
		machineStatus := omni.NewMachineStatus(name)
		machineStatus.Metadata().Annotations().Set(omni.KernelArgsInitialized, "")
		machineStatus.TypedSpec().Value.TalosVersion = talosVersion
		machineStatus.TypedSpec().Value.InitialTalosVersion = talosVersion
		machineStatus.TypedSpec().Value.Schematic = &specs.MachineStatusSpec_Schematic{
			FullId:           rawID,
			Raw:              string(rawYAML),
			Extensions:       []string{"siderolabs/hello-world-service"},
			KernelArgs:       []string{"console=ttyS0"},
			InitialSchematic: rawID,
			InitialState: &specs.MachineStatusSpec_Schematic_InitialState{
				Extensions: []string{"siderolabs/hello-world-service"},
			},
		}
		machineStatus.TypedSpec().Value.SecurityState = &specs.SecurityState{}
		machineStatus.TypedSpec().Value.PlatformMetadata = &specs.MachineStatusSpec_PlatformMetadata{
			Platform: talosconstants.PlatformMetal,
		}

		return machineStatus
	}

	for _, tt := range []struct {
		name string
		// recordedID is the schematic ID seeded into the SchematicConfiguration, without a factory, empty for
		// a machine without one
		recordedID  string
		expectedID  string
		expectedURL string
		// secondaryKnows makes the secondary factory the one that issued the recorded ID
		secondaryKnows bool
	}{
		{
			name:        "records the factory that issued the schematic",
			expectedID:  rawID,
			expectedURL: primaryURL,
		},
		{
			name:           "finds the factory of a schematic recorded without one",
			recordedID:     rawID,
			secondaryKnows: true,
			expectedID:     rawID,
			expectedURL:    secondaryURL,
		},
		{
			name:        "keeps the schematic when no factory knows it",
			recordedID:  rawID,
			expectedID:  rawID,
			expectedURL: "",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()

			primary := &testutils.ImageFactoryClientMock{FactoryURL: primaryURL}
			secondary := &testutils.ImageFactoryClientMock{FactoryURL: secondaryURL}

			if tt.secondaryKnows {
				_, _, err = secondary.EnsureSchematic(ctx, raw)
				require.NoError(t, err)
			}

			const machineName = "machine"

			testutils.WithRuntime(
				ctx, t, testutils.TestOptions{},
				func(ctx context.Context, testContext testutils.TestContext) {
					require.NoError(t, testContext.Runtime.RegisterQController(schematicctrl.NewConfigurationController(testutils.NewFactoryClientSet(primary, secondary))))
					require.NoError(t, testContext.Runtime.RegisterQController(omnictrl.NewMachineExtensionsController()))

					require.NoError(t, testContext.State.Create(ctx, newMachineStatus(machineName)))

					if tt.recordedID != "" {
						schematicConfiguration := omni.NewSchematicConfiguration(machineName)
						schematicConfiguration.TypedSpec().Value.SchematicId = tt.recordedID
						schematicConfiguration.TypedSpec().Value.TalosVersion = talosVersion

						require.NoError(t, testContext.State.Create(ctx, schematicConfiguration, state.WithCreateOwner(schematicctrl.ConfigurationControllerName)))
					}
				},
				func(ctx context.Context, testContext testutils.TestContext) {
					// The extensions status is written at the end of every reconcile, so its presence means the
					// schematic configuration above has been looked at.
					rtestutils.AssertResources(ctx, t, testContext.State, []string{machineName}, func(*omni.MachineExtensionsStatus, *assert.Assertions) {})

					rtestutils.AssertResources(ctx, t, testContext.State, []string{machineName}, func(res *omni.SchematicConfiguration, assertion *assert.Assertions) {
						assertion.Equal(tt.expectedID, res.TypedSpec().Value.SchematicId)
						assertion.Equal(tt.expectedURL, res.TypedSpec().Value.ImageFactoryUrl)
					})

					if tt.recordedID != "" {
						_, ensuredOnPrimary := primary.Get(rawID)
						assert.False(t, ensuredOnPrimary, "a recorded schematic must not be re-ensured on the primary")
					}
				},
			)
		})
	}
}
