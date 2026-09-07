// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"context"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/imagefactory"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils/rmock"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils/rmock/options"
)

// TestMachineConfigGenOptionsFactoryHost covers which image factory host the controller records in the
// install image of a machine that already has MachineConfigGenOptions, i.e. what an Omni upgrade leaves
// behind.
//
// Regression test for https://github.com/siderolabs/omni/issues/3247: machines enrolled before Omni
// started tracking the factory host per machine have it empty, and the controller kept carrying that
// empty value forward, so every install of such a machine failed with "has no image factory host set".
//
// The host is the factory that issued the schematic in use, recorded on the SchematicConfiguration for the
// Talos version it was ensured for. While that record does not cover the schematic and version in use, the
// install image already published for that same schematic and version keeps its host, and with neither
// nothing is published: a guessed factory would only fail the pull.
//
//nolint:maintidx
func TestMachineConfigGenOptionsFactoryHost(t *testing.T) {
	t.Parallel()

	const (
		primaryFactoryHost   = "primary.factory.test"
		secondaryFactoryHost = "secondary.factory.test"
		recordedFactoryHost  = "recorded.factory.test"

		installedTalosVersion = "1.9.3"
		upgradeTalosVersion   = "1.10.0"
	)

	// installImage is the install image of the machine as it is stored in the state before the
	// controller reconciles it.
	installImage := func(talosVersion, factoryHost string) *specs.MachineConfigGenOptionsSpec_InstallImage {
		return &specs.MachineConfigGenOptionsSpec_InstallImage{
			TalosVersion:         talosVersion,
			SchematicId:          defaultSchematic,
			SchematicInitialized: true,
			Platform:             "metal",
			SecurityState:        &specs.SecurityState{},
			ImageFactoryHost:     factoryHost,
		}
	}

	// schematicRecord is the SchematicConfiguration of the machine: the schematic ID, the Talos version it was
	// ensured for, and the factory that issued it.
	schematicRecord := func(schematicID, talosVersion, factoryHost string) *specs.SchematicConfigurationSpec {
		return &specs.SchematicConfigurationSpec{
			SchematicId:     schematicID,
			TalosVersion:    talosVersion,
			ImageFactoryUrl: "https://" + factoryHost,
		}
	}

	for _, tt := range []struct {
		storedInstallImage *specs.MachineConfigGenOptionsSpec_InstallImage
		schematicRecord    *specs.SchematicConfigurationSpec
		name               string
		// the Talos version the machine is allocated with, empty when the machine is not allocated to a cluster
		allocatedTalosVersion string
		expectedFactoryHost   string
		// allocatedSchematic is the schematic ID the allocation names, defaultSchematic when empty
		allocatedSchematic string
		withSecondary      bool
		// invalidSchematic allocates the machine with no schematic ID at all (a machine that bypassed the
		// factory), which needs no factory host
		invalidSchematic bool
		// expectUntouched expects the stored install image to be left exactly as it was
		expectUntouched bool
	}{
		{
			// the issue: Omni 1.9.x recorded no factory host, and the machine is now back out of its cluster
			name:                "fills the empty factory host of a machine that is not allocated from the schematic's record",
			storedInstallImage:  installImage(installedTalosVersion, ""),
			schematicRecord:     schematicRecord(defaultSchematic, installedTalosVersion, secondaryFactoryHost),
			withSecondary:       true,
			expectedFactoryHost: secondaryFactoryHost,
		},
		{
			name:                "leaves the empty factory host of a machine that is not allocated alone without a record",
			storedInstallImage:  installImage(installedTalosVersion, ""),
			withSecondary:       true,
			expectedFactoryHost: "",
		},
		{
			name:                "keeps the recorded factory host of a machine that is not allocated",
			storedInstallImage:  installImage(installedTalosVersion, recordedFactoryHost),
			schematicRecord:     schematicRecord(defaultSchematic, installedTalosVersion, primaryFactoryHost),
			withSecondary:       true,
			expectedFactoryHost: recordedFactoryHost,
		},
		{
			// neither the schematic nor the install image records a factory: nothing is published until one does,
			// a guessed factory would only fail the pull
			name:                  "waits for a factory when neither the schematic nor the install image records one",
			storedInstallImage:    installImage(installedTalosVersion, ""),
			allocatedTalosVersion: installedTalosVersion,
			withSecondary:         true,
			expectedFactoryHost:   "",
		},
		{
			// a machine allocated after a factory switch: its schematic was issued by the factory that is the
			// secondary now, and only that factory knows the ID
			name:                  "names the factory that issued the schematic, not the one serving the version",
			storedInstallImage:    installImage(installedTalosVersion, ""),
			schematicRecord:       schematicRecord(defaultSchematic, installedTalosVersion, secondaryFactoryHost),
			allocatedTalosVersion: installedTalosVersion,
			withSecondary:         true,
			expectedFactoryHost:   secondaryFactoryHost,
		},
		{
			name:                  "keeps the recorded factory host of an allocated machine",
			storedInstallImage:    installImage(installedTalosVersion, recordedFactoryHost),
			allocatedTalosVersion: installedTalosVersion,
			expectedFactoryHost:   recordedFactoryHost,
		},
		{
			// the schematic was ensured on the primary for the new version before the allocation moved
			name:                  "moves the factory host with the schematic when the Talos version changes",
			storedInstallImage:    installImage(installedTalosVersion, recordedFactoryHost),
			schematicRecord:       schematicRecord(defaultSchematic, upgradeTalosVersion, primaryFactoryHost),
			allocatedTalosVersion: upgradeTalosVersion,
			withSecondary:         true,
			expectedFactoryHost:   primaryFactoryHost,
		},
		{
			// the record covers another schematic ID: the allocation has not caught up with it yet
			name:                  "keeps the recorded factory host while the schematic record covers another ID",
			storedInstallImage:    installImage(installedTalosVersion, recordedFactoryHost),
			schematicRecord:       schematicRecord("other-schematic", installedTalosVersion, primaryFactoryHost),
			allocatedTalosVersion: installedTalosVersion,
			withSecondary:         true,
			expectedFactoryHost:   recordedFactoryHost,
		},
		{
			// the record is for the version the machine runs, the allocation moved on to the next one: the
			// install image published for the old version is left as it is until the record catches up
			name:                  "waits while the schematic record covers another Talos version",
			storedInstallImage:    installImage(installedTalosVersion, recordedFactoryHost),
			schematicRecord:       schematicRecord(defaultSchematic, installedTalosVersion, primaryFactoryHost),
			allocatedTalosVersion: upgradeTalosVersion,
			withSecondary:         true,
			expectedFactoryHost:   recordedFactoryHost,
			expectUntouched:       true,
		},
		{
			// the allocation already names a new schematic the record does not cover yet: the host of the
			// old schematic must not be paired with the new ID
			name:                  "waits while the allocation names a schematic the record does not cover",
			storedInstallImage:    installImage(installedTalosVersion, recordedFactoryHost),
			schematicRecord:       schematicRecord(defaultSchematic, installedTalosVersion, primaryFactoryHost),
			allocatedTalosVersion: installedTalosVersion,
			allocatedSchematic:    "new-schematic",
			withSecondary:         true,
			expectedFactoryHost:   recordedFactoryHost,
			expectUntouched:       true,
		},
		{
			name:                  "keeps the recorded factory host when the record names a factory that is not configured",
			storedInstallImage:    installImage(installedTalosVersion, recordedFactoryHost),
			schematicRecord:       schematicRecord(defaultSchematic, installedTalosVersion, "gone.factory.test"),
			allocatedTalosVersion: installedTalosVersion,
			expectedFactoryHost:   recordedFactoryHost,
		},
		{
			name:                  "publishes the install image of a machine without a schematic with no factory host",
			storedInstallImage:    nil,
			allocatedTalosVersion: installedTalosVersion,
			invalidSchematic:      true,
			withSecondary:         true,
			expectedFactoryHost:   "",
		},
		{
			name:                  "waits for a factory for a machine that never had an install image",
			storedInstallImage:    nil,
			allocatedTalosVersion: installedTalosVersion,
			withSecondary:         true,
			expectedFactoryHost:   "",
		},
		{
			name:                  "publishes the install image of a machine that never had one from the schematic's record",
			storedInstallImage:    nil,
			schematicRecord:       schematicRecord(defaultSchematic, installedTalosVersion, primaryFactoryHost),
			allocatedTalosVersion: installedTalosVersion,
			withSecondary:         true,
			expectedFactoryHost:   primaryFactoryHost,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			t.Cleanup(cancel)

			// The machine under test, and a second "fence" machine seeded with an empty factory
			// host, so that its backfill is always an observable write. Both are seeded before
			// the runtime starts, the initial reconcile queue is ordered by ID, and the
			// controller runs its default single worker. Therefore, once the fence machine is
			// backfilled, the machine under test has been reconciled too. This way, the
			// keep-the-host cases assert on the reconciled value, not on the seeded one, as a
			// no-change reconcile leaves no trace of its own on the resource.
			const (
				machineID      = "machine-1"
				fenceMachineID = "machine-2"
			)

			require.Less(t, machineID, fenceMachineID, "the fence works only when the machine under test is reconciled first")

			testutils.WithRuntime(
				ctx, t, testutils.TestOptions{},
				func(ctx context.Context, tc testutils.TestContext) {
					primary, err := imagefactory.NewClient("https://"+primaryFactoryHost, "", "")
					require.NoError(t, err)

					// no TalosVersion resources are created, so ForTalosVersion always resolves to the primary
					clients := imagefactory.NewClients(tc.State, primary)

					if tt.withSecondary {
						var secondary *imagefactory.Client

						secondary, err = imagefactory.NewClient("https://"+secondaryFactoryHost, "", "")
						require.NoError(t, err)

						clients.SetSecondary(secondary)
					}

					require.NoError(t, tc.Runtime.RegisterQController(omnictrl.NewMachineConfigGenOptionsController(clients)))

					// Seed the resources before the runtime starts: the controller has to see the state left
					// behind by an Omni upgrade instead of generating the options from scratch.
					rmock.Mock[*omni.MachineStatus](ctx, t, tc.State, options.WithID(machineID))

					// the machine is allocated to a cluster exactly while it has a ClusterMachineTalosVersion
					if tt.allocatedTalosVersion != "" {
						rmock.Mock[*omni.ClusterMachineTalosVersion](
							ctx, t, tc.State, options.WithID(machineID),
							options.Modify(func(res *omni.ClusterMachineTalosVersion) error {
								res.TypedSpec().Value.TalosVersion = tt.allocatedTalosVersion

								switch {
								case tt.invalidSchematic:
								case tt.allocatedSchematic != "":
									res.TypedSpec().Value.SchematicId = tt.allocatedSchematic
								default:
									res.TypedSpec().Value.SchematicId = defaultSchematic
								}

								return nil
							}),
						)
					}

					rmock.Mock[*omni.MachineConfigGenOptions](
						ctx, t, tc.State, options.WithID(machineID),
						options.Modify(func(res *omni.MachineConfigGenOptions) error {
							res.TypedSpec().Value.InstallImage = tt.storedInstallImage.CloneVT()

							return nil
						}),
					)

					if tt.schematicRecord != nil {
						rmock.Mock[*omni.SchematicConfiguration](
							ctx, t, tc.State, options.WithID(machineID),
							options.Modify(func(res *omni.SchematicConfiguration) error {
								res.TypedSpec().Value = tt.schematicRecord.CloneVT()

								return nil
							}),
						)
					}

					// The fence machine's schematic record names the primary, so its empty host gets filled.
					rmock.Mock[*omni.MachineStatus](ctx, t, tc.State, options.WithID(fenceMachineID))
					rmock.Mock[*omni.MachineConfigGenOptions](
						ctx, t, tc.State, options.WithID(fenceMachineID),
						options.Modify(func(res *omni.MachineConfigGenOptions) error {
							res.TypedSpec().Value.InstallImage = installImage(installedTalosVersion, "")

							return nil
						}),
					)
					rmock.Mock[*omni.SchematicConfiguration](
						ctx, t, tc.State, options.WithID(fenceMachineID),
						options.Modify(func(res *omni.SchematicConfiguration) error {
							res.TypedSpec().Value = schematicRecord(defaultSchematic, installedTalosVersion, primaryFactoryHost)

							return nil
						}),
					)
				},
				func(ctx context.Context, tc testutils.TestContext) {
					rtestutils.AssertResource(ctx, t, tc.State, fenceMachineID, func(res *omni.MachineConfigGenOptions, assertions *assert.Assertions) {
						assertions.Equal(primaryFactoryHost, res.TypedSpec().Value.InstallImage.GetImageFactoryHost())
					})

					rtestutils.AssertResource(ctx, t, tc.State, machineID, func(res *omni.MachineConfigGenOptions, assertions *assert.Assertions) {
						assertions.Equal(tt.expectedFactoryHost, res.TypedSpec().Value.InstallImage.GetImageFactoryHost())

						if tt.invalidSchematic {
							assertions.NotNil(res.TypedSpec().Value.InstallImage, "a machine without a schematic needs no factory host to get its install image")
						}

						if tt.expectUntouched {
							assertions.True(tt.storedInstallImage.EqualVT(res.TypedSpec().Value.InstallImage), "the stored install image must be left as it is: %v", res.TypedSpec().Value.InstallImage)
						}
					})
				},
			)
		})
	}
}
