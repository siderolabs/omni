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
	"github.com/siderolabs/talos/pkg/machinery/api/storage"
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

func TestGenInstallConfig(t *testing.T) {
	for _, tt := range []struct {
		name                string
		talosVersion        string
		machineStatusSpec   *specs.MachineStatusSpec
		expectedInstallDisk string
	}{
		{
			name: "empty",
		},
		{
			name:              "nohw",
			machineStatusSpec: &specs.MachineStatusSpec{},
		},
		{
			name: "single disk",
			machineStatusSpec: &specs.MachineStatusSpec{
				Hardware: &specs.MachineStatusSpec_HardwareStatus{
					Blockdevices: []*specs.MachineStatusSpec_HardwareStatus_BlockDevice{
						{
							LinuxName: "/dev/sda",
							Size:      8e9,
						},
					},
				},
			},
			expectedInstallDisk: "/dev/sda",
		},
		{
			name: "not matched",
			machineStatusSpec: &specs.MachineStatusSpec{
				Hardware: &specs.MachineStatusSpec_HardwareStatus{
					Blockdevices: []*specs.MachineStatusSpec_HardwareStatus_BlockDevice{
						{
							LinuxName: "/dev/sda",
							Size:      4e9,
						},
						{
							LinuxName: "/dev/sda",
							Size:      8e9,
							Type:      storage.Disk_CD.String(),
						},
						{
							LinuxName: "/dev/sda",
							Size:      4e9,
						},
					},
				},
			},
		},
		{
			name: "matched not usb not virtual",
			machineStatusSpec: &specs.MachineStatusSpec{
				Hardware: &specs.MachineStatusSpec_HardwareStatus{
					Blockdevices: []*specs.MachineStatusSpec_HardwareStatus_BlockDevice{
						{
							LinuxName: "/dev/sda",
							Size:      8e9,
							Transport: "usb",
						},
						{
							LinuxName: "/dev/dm-0",
							Size:      8e9,
							BusPath:   "/virtual",
						},
						{
							LinuxName: "/dev/sdb",
							Size:      10e9,
						},
						{
							LinuxName: "/dev/sdc",
							Size:      14e9,
						},
						{
							LinuxName: "/dev/sdf",
							Size:      7e9,
							Transport: "usb",
						},
						{
							LinuxName: "/dev/dm-1",
							Size:      7e9,
							BusPath:   "/virtual",
						},
					},
				},
			},
			expectedInstallDisk: "/dev/sdb",
		},
		{
			name: "select by size",
			machineStatusSpec: &specs.MachineStatusSpec{
				Hardware: &specs.MachineStatusSpec_HardwareStatus{
					Blockdevices: []*specs.MachineStatusSpec_HardwareStatus_BlockDevice{
						{
							Size:      25165824000,
							LinuxName: "/dev/sda",
							Transport: "sata",
							Type:      "HDD",
						},
						{
							Size:      6442450944,
							LinuxName: "/dev/vdb",
							Transport: "usb",
							Type:      "HDD",
						},
						{
							Size:      6442450944,
							LinuxName: "/dev/vda",
							Transport: "virtio",
							Type:      "HDD",
						},
						{
							Size:      6442450943,
							LinuxName: "/dev/vdc",
							Transport: "usb",
							Type:      "HDD",
						},
					},
				},
			},
			expectedInstallDisk: "/dev/vda",
		},
		{
			name: "system disk",
			machineStatusSpec: &specs.MachineStatusSpec{
				Hardware: &specs.MachineStatusSpec_HardwareStatus{
					Blockdevices: []*specs.MachineStatusSpec_HardwareStatus_BlockDevice{
						{
							LinuxName: "/dev/sda",
							Size:      8e9,
						},
						{
							LinuxName:  "/dev/sdb",
							Size:       10e9,
							SystemDisk: true,
						},
						{
							LinuxName: "/dev/sdc",
							Size:      14e9,
						},
					},
				},
			},
			expectedInstallDisk: "/dev/sdb",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ms := omni.NewMachineStatus("id")

			if tt.machineStatusSpec != nil {
				ms.TypedSpec().Value = tt.machineStatusSpec
			}

			talosVersion := omni.NewClusterMachineTalosVersion("id")
			talosVersion.TypedSpec().Value.TalosVersion = tt.talosVersion

			genOptions := omni.NewMachineConfigGenOptions("id")

			omnictrl.GenInstallConfig(ms, talosVersion, genOptions, "factory.talos.dev")

			assert.Equal(t, tt.expectedInstallDisk, genOptions.TypedSpec().Value.InstallDisk)
		})
	}
}

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

		// the install disk the controller picks from the mocked machine status hardware
		expectedInstallDisk = "sda"
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

			const machineID = "machine-1"

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
							res.TypedSpec().Value.InstallDisk = ""
							res.TypedSpec().Value.InstallImage = tt.storedInstallImage.CloneVT()

							return nil
						}),
					)
				},
				func(ctx context.Context, tc testutils.TestContext) {
					rtestutils.AssertResource(ctx, t, tc.State, machineID, func(res *omni.MachineConfigGenOptions, assertions *assert.Assertions) {
						// the install disk is only ever written by the controller, so it marks the resource as reconciled
						assertions.Equal(expectedInstallDisk, res.TypedSpec().Value.InstallDisk)
						assertions.Equal(tt.expectedFactoryHost, res.TypedSpec().Value.InstallImage.GetImageFactoryHost())
					})
				},
			)
		})
	}
}
