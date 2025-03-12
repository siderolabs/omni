// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"testing"

	"github.com/siderolabs/talos/pkg/machinery/api/storage"
	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
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
			name: "matched not usb",
			machineStatusSpec: &specs.MachineStatusSpec{
				Hardware: &specs.MachineStatusSpec_HardwareStatus{
					Blockdevices: []*specs.MachineStatusSpec_HardwareStatus_BlockDevice{
						{
							LinuxName: "/dev/sda",
							Size:      8e9,
							Transport: "usb",
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
					},
				},
			},
			expectedInstallDisk: "/dev/sdb",
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
			ms := omni.NewMachineStatus(resources.DefaultNamespace, "id")

			if tt.machineStatusSpec != nil {
				ms.TypedSpec().Value = tt.machineStatusSpec
			}

			talosVersion := omni.NewClusterMachineTalosVersion(resources.DefaultNamespace, "id")
			talosVersion.TypedSpec().Value.TalosVersion = tt.talosVersion

			genOptions := omni.NewMachineConfigGenOptions(resources.DefaultNamespace, "id")

			omnictrl.GenInstallConfig(ms, talosVersion, genOptions)

			assert.Equal(t, tt.expectedInstallDisk, genOptions.TypedSpec().Value.InstallDisk)
		})
	}
}
