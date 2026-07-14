// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package lifecycle_test

import (
	"testing"

	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/talos/lifecycle"
)

func TestDecideOp(t *testing.T) {
	t.Parallel()

	mkMachineStatus := func(version string, schematic *specs.MachineStatusSpec_Schematic, hasSystemDisk, inMaintenance bool) *omni.MachineStatus {
		ms := omni.NewMachineStatus("m1")

		ms.TypedSpec().Value.TalosVersion = version
		ms.TypedSpec().Value.Schematic = schematic

		ms.TypedSpec().Value.Maintenance = inMaintenance

		if hasSystemDisk {
			ms.TypedSpec().Value.Hardware = &specs.MachineStatusSpec_HardwareStatus{
				Blockdevices: []*specs.MachineStatusSpec_HardwareStatus_BlockDevice{
					{LinuxName: "/dev/sda", SystemDisk: true},
				},
			}
		}

		return ms
	}

	mkSnapshot := func(stage machineapi.MachineStatusEvent_MachineStage) *omni.MachineStatusSnapshot {
		s := omni.NewMachineStatusSnapshot("m1")
		s.TypedSpec().Value.MachineStatus = &machineapi.MachineStatusEvent{Stage: stage}

		return s
	}

	mkImage := func(targetVersion, schematic string) *specs.MachineConfigGenOptionsSpec_InstallImage {
		return &specs.MachineConfigGenOptionsSpec_InstallImage{TalosVersion: targetVersion, SchematicId: schematic}
	}

	type tcase struct {
		machine              *omni.MachineStatus
		snapshot             *omni.MachineStatusSnapshot
		image                *specs.MachineConfigGenOptionsSpec_InstallImage
		name                 string
		want                 lifecycle.Op
		schematicMismatch    bool
		talosVersionMismatch bool
	}

	for _, tc := range []tcase{
		{
			name:     "in sync, nothing to do",
			machine:  mkMachineStatus("1.13.4", nil, true, false),
			snapshot: mkSnapshot(machineapi.MachineStatusEvent_RUNNING),
			image:    mkImage("1.13.4", ""),
			want:     lifecycle.OpNone,
		},
		{
			name:                 "not-installed 1.13 in maintenance, minor version mismatch → maintenance install",
			machine:              mkMachineStatus("1.13.0", nil, false, true),
			snapshot:             mkSnapshot(machineapi.MachineStatusEvent_MAINTENANCE),
			image:                mkImage("1.14.1", ""),
			talosVersionMismatch: true,
			want:                 lifecycle.OpMaintenanceInstall,
		},
		{
			name:                 "not-installed 1.13 in maintenance, patch-only mismatch → config-apply install (None)",
			machine:              mkMachineStatus("1.13.0", nil, false, true),
			snapshot:             mkSnapshot(machineapi.MachineStatusEvent_MAINTENANCE),
			image:                mkImage("1.13.4", ""),
			talosVersionMismatch: true,
			// same config contract (major.minor): the maintenance config apply installs the exact target version itself, no lifecycle install needed
			want: lifecycle.OpNone,
		},
		{
			name:     "not-installed 1.13 in maintenance at target version → config-apply install (None) despite stale mismatch flags",
			machine:  mkMachineStatus("1.13.4", nil, false, true),
			snapshot: mkSnapshot(machineapi.MachineStatusEvent_MAINTENANCE),
			image:    mkImage("1.13.4", ""),
			// machineConfigStatus-based flags signal a mismatch on the first reconcile (empty status); the live machine state must win.
			schematicMismatch:    true,
			talosVersionMismatch: true,
			want:                 lifecycle.OpNone,
		},
		{
			name:                 "not-installed 1.13 in maintenance, schematic-only mismatch → config-apply install (None)",
			machine:              mkMachineStatus("1.13.4", &specs.MachineStatusSpec_Schematic{FullId: "boot-schematic"}, false, true),
			snapshot:             mkSnapshot(machineapi.MachineStatusEvent_MAINTENANCE),
			image:                mkImage("1.13.4", "target-schematic"),
			schematicMismatch:    true,
			talosVersionMismatch: true,
			want:                 lifecycle.OpNone,
		},
		{
			name:                 "installed 1.13 in maintenance with patch-only mismatch → maintenance upgrade (exact compare)",
			machine:              mkMachineStatus("1.13.0", nil, true, true),
			snapshot:             mkSnapshot(machineapi.MachineStatusEvent_MAINTENANCE),
			image:                mkImage("1.13.4", ""),
			talosVersionMismatch: true,
			// config apply cannot change the on-disk version, so installed machines compare exact versions, not just the contract
			want: lifecycle.OpMaintenanceUpgrade,
		},
		{
			name:     "installed 1.13 in maintenance with schematic mismatch → maintenance upgrade",
			machine:  mkMachineStatus("1.13.4", &specs.MachineStatusSpec_Schematic{FullId: "boot-schematic"}, true, true),
			snapshot: mkSnapshot(machineapi.MachineStatusEvent_MAINTENANCE),
			image:    mkImage("1.13.4", "target-schematic"),
			want:     lifecycle.OpMaintenanceUpgrade,
		},
		{
			name:     "installed 1.13 in maintenance at target → config apply (None), the post-install state",
			machine:  mkMachineStatus("1.13.4", &specs.MachineStatusSpec_Schematic{FullId: "target-schematic"}, true, true),
			snapshot: mkSnapshot(machineapi.MachineStatusEvent_MAINTENANCE),
			image:    mkImage("1.13.4", "target-schematic"),
			// stale flags again: the maintenance flow never writes the config status back
			schematicMismatch:    true,
			talosVersionMismatch: true,
			want:                 lifecycle.OpNone,
		},
		{
			name:                 "installed 1.12 in maintenance → legacy upgrade (machine doesn't support lifecycle)",
			machine:              mkMachineStatus("1.12.5", nil, true, true),
			snapshot:             mkSnapshot(machineapi.MachineStatusEvent_MAINTENANCE),
			image:                mkImage("1.12.6", ""),
			talosVersionMismatch: true,
			want:                 lifecycle.OpLegacyUpgrade,
		},
		{
			name:                 "installed 1.13 running mode mismatch → cluster upgrade (both support lifecycle)",
			machine:              mkMachineStatus("1.13.4", nil, true, false),
			snapshot:             mkSnapshot(machineapi.MachineStatusEvent_RUNNING),
			image:                mkImage("1.13.5", ""),
			talosVersionMismatch: true,
			want:                 lifecycle.OpClusterUpgrade,
		},
		{
			name:                 "installed 1.12 running mode mismatch → legacy upgrade (machine doesn't support lifecycle)",
			machine:              mkMachineStatus("1.12.5", nil, true, false),
			snapshot:             mkSnapshot(machineapi.MachineStatusEvent_RUNNING),
			image:                mkImage("1.12.6", ""),
			talosVersionMismatch: true,
			want:                 lifecycle.OpLegacyUpgrade,
		},
		{
			name:                 "installed 1.13 running, downgrade to 1.12 → legacy upgrade (target doesn't support lifecycle)",
			machine:              mkMachineStatus("1.13.4", nil, true, false),
			snapshot:             mkSnapshot(machineapi.MachineStatusEvent_RUNNING),
			image:                mkImage("1.12.6", ""),
			talosVersionMismatch: true,
			want:                 lifecycle.OpLegacyUpgrade,
		},
		{
			name:                 "not-installed 1.12 in maintenance + 1.13 cluster → no path (gate stands)",
			machine:              mkMachineStatus("1.12.5", nil, false, true),
			snapshot:             mkSnapshot(machineapi.MachineStatusEvent_MAINTENANCE),
			image:                mkImage("1.13.4", ""),
			talosVersionMismatch: true,
			want:                 lifecycle.OpNone,
		},
		{
			name:                 "not-installed 1.13 in maintenance + 1.12 cluster → no path (target doesn't support lifecycle)",
			machine:              mkMachineStatus("1.13.4", nil, false, true),
			snapshot:             mkSnapshot(machineapi.MachineStatusEvent_MAINTENANCE),
			image:                mkImage("1.12.6", ""),
			talosVersionMismatch: true,
			want:                 lifecycle.OpNone,
		},
		{
			name:    "invalid schematic plays no role in the maintenance decision",
			machine: mkMachineStatus("1.13.4", &specs.MachineStatusSpec_Schematic{Invalid: true}, true, true),
			snapshot: mkSnapshot(
				machineapi.MachineStatusEvent_MAINTENANCE,
			),
			image: mkImage("1.13.4", "target-schematic"),
			want:  lifecycle.OpNone,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := lifecycle.DecideOp(tc.machine, tc.image, tc.schematicMismatch, tc.talosVersionMismatch)
			assert.Equal(t, tc.want, got)
		})
	}
}
