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
// The host of a machine that is allocated to a cluster is left alone: its install image describes what
// the machine is running, and is only ever rewritten by the allocation itself. The empty host is
// backfilled while the machine is free, which is the state it passes through on its way back into a
// cluster.
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

	for _, tt := range []struct {
		storedInstallImage *specs.MachineConfigGenOptionsSpec_InstallImage
		name               string
		// the Talos version the machine is allocated with, empty when the machine is not allocated to a cluster
		allocatedTalosVersion string
		expectedFactoryHost   string
		withSecondary         bool
	}{
		{
			// the issue: Omni 1.9.x recorded no factory host, and the machine is now back out of its cluster
			name:                "backfills the empty factory host of a machine that is not allocated",
			storedInstallImage:  installImage(installedTalosVersion, ""),
			expectedFactoryHost: primaryFactoryHost,
		},
		{
			// the secondary factory is the one Omni is migrating away from, so it is the factory the
			// schematic of a machine enrolled before the upgrade was created on
			name:                "backfills the empty factory host of a machine that is not allocated from the secondary factory",
			storedInstallImage:  installImage(installedTalosVersion, ""),
			withSecondary:       true,
			expectedFactoryHost: secondaryFactoryHost,
		},
		{
			name:                "keeps the recorded factory host of a machine that is not allocated",
			storedInstallImage:  installImage(installedTalosVersion, recordedFactoryHost),
			withSecondary:       true,
			expectedFactoryHost: recordedFactoryHost,
		},
		{
			// the install image of an allocated machine is left as it is: the machine is running it, and the
			// install/upgrade path resolves the factory on its own when the host is empty
			name:                  "leaves the empty factory host of an allocated machine alone",
			storedInstallImage:    installImage(installedTalosVersion, ""),
			allocatedTalosVersion: installedTalosVersion,
			expectedFactoryHost:   "",
		},
		{
			name:                  "keeps the recorded factory host of an allocated machine",
			storedInstallImage:    installImage(installedTalosVersion, recordedFactoryHost),
			allocatedTalosVersion: installedTalosVersion,
			expectedFactoryHost:   recordedFactoryHost,
		},
		{
			name:                  "upgrades the factory host when the Talos version changes",
			storedInstallImage:    installImage(installedTalosVersion, recordedFactoryHost),
			allocatedTalosVersion: upgradeTalosVersion,
			expectedFactoryHost:   primaryFactoryHost,
		},
		{
			name:                  "uses the primary factory for a machine that never had an install image",
			storedInstallImage:    nil,
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
								res.TypedSpec().Value.SchematicId = defaultSchematic

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

					rmock.Mock[*omni.MachineStatus](ctx, t, tc.State, options.WithID(fenceMachineID))
					rmock.Mock[*omni.MachineConfigGenOptions](
						ctx, t, tc.State, options.WithID(fenceMachineID),
						options.Modify(func(res *omni.MachineConfigGenOptions) error {
							res.TypedSpec().Value.InstallImage = installImage(installedTalosVersion, "")

							return nil
						}),
					)
				},
				func(ctx context.Context, tc testutils.TestContext) {
					fenceFactoryHost := primaryFactoryHost
					if tt.withSecondary {
						fenceFactoryHost = secondaryFactoryHost
					}

					rtestutils.AssertResource(ctx, t, tc.State, fenceMachineID, func(res *omni.MachineConfigGenOptions, assertions *assert.Assertions) {
						assertions.Equal(fenceFactoryHost, res.TypedSpec().Value.InstallImage.GetImageFactoryHost())
					})

					rtestutils.AssertResource(ctx, t, tc.State, machineID, func(res *omni.MachineConfigGenOptions, assertions *assert.Assertions) {
						assertions.Equal(tt.expectedFactoryHost, res.TypedSpec().Value.InstallImage.GetImageFactoryHost())
					})
				},
			)
		})
	}
}
