// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package lifecycle

import (
	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// Op identifies which upgrade/install path a controller should follow for a given machine.
type Op int

const (
	// OpNone means on-disk Talos already matches the install image; no action needed.
	OpNone Op = iota

	// OpLegacyUpgrade is an in-place upgrade via MachineService.Upgrade.
	OpLegacyUpgrade

	// OpMaintenanceInstall is LifecycleService.Install for a maintenance machine with no Talos on disk.
	OpMaintenanceInstall

	// OpMaintenanceUpgrade is LifecycleService.Upgrade for a maintenance machine that has Talos on disk.
	OpMaintenanceUpgrade

	// OpClusterUpgrade is LifecycleService.Upgrade for an in-cluster machine (Talos 1.13+), with cordon/drain/reboot orchestrated by Omni.
	OpClusterUpgrade
)

// DecideOp picks the upgrade/install path from the live machine state.
func DecideOp(machineStatus *omni.MachineStatus, installImage *specs.MachineConfigGenOptionsSpec_InstallImage, schematicMismatch, talosVersionMismatch bool) Op {
	hasSystemDisk := omni.GetMachineStatusSystemDisk(machineStatus) != ""

	machineVersion, machineSupportsLifecycle := omni.ParseTalosVersionLifecycleSupport(machineStatus.TypedSpec().Value.TalosVersion)
	targetVersion, targetSupportsLifecycle := omni.ParseTalosVersionLifecycleSupport(installImage.TalosVersion)

	// The LifecycleService paths (Talos 1.13+ both ends) decide from the live version/schematic, not the
	// mismatch flags, so a stale status can't trigger a spurious first-reconcile upgrade.
	if machineSupportsLifecycle && targetSupportsLifecycle {
		if machineStatus.TypedSpec().Value.Maintenance {
			if hasSystemDisk {
				if !machineVersion.EQ(targetVersion) || schematicDiffers(machineStatus, installImage) {
					return OpMaintenanceUpgrade
				}

				return OpNone
			}

			// No Talos on disk: always install explicitly via LifecycleService.Install. ApplyConfig is used
			// only to configure machines that already have Talos on disk, never to trigger an install.
			return OpMaintenanceInstall
		}

		if hasSystemDisk {
			if !machineVersion.EQ(targetVersion) || schematicDiffers(machineStatus, installImage) {
				return OpClusterUpgrade
			}

			return OpNone
		}
	}

	// Older machine or target: the legacy path, gated on the config-status mismatch flags.
	if !schematicMismatch && !talosVersionMismatch {
		return OpNone
	}

	if hasSystemDisk {
		return OpLegacyUpgrade
	}

	// No Talos on disk and not eligible for a maintenance install: Omni has no way to install Talos on this machine today, so nothing is done.
	return OpNone
}

// schematicDiffers reports whether the machine's schematic differs from the target's. An invalid schematic
// (not provisioned via image factory) reports false.
func schematicDiffers(machineStatus *omni.MachineStatus, installImage *specs.MachineConfigGenOptionsSpec_InstallImage) bool {
	machineSchematic := machineStatus.TypedSpec().Value.GetSchematic()

	return !machineSchematic.GetInvalid() && machineSchematic.GetFullId() != installImage.SchematicId
}
