// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"context"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/controller/runtime"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/talos/pkg/machinery/api/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

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
			ms := omni.NewMachineStatus(resources.DefaultNamespace, "id")

			if tt.machineStatusSpec != nil {
				ms.TypedSpec().Value = tt.machineStatusSpec
			}

			talosVersion := omni.NewClusterMachineTalosVersion(resources.DefaultNamespace, "id")
			talosVersion.TypedSpec().Value.TalosVersion = tt.talosVersion

			genOptions := omni.NewMachineConfigGenOptions(resources.DefaultNamespace, "id")

			omnictrl.GenInstallConfig(ms, talosVersion, genOptions, nil, true)

			assert.Equal(t, tt.expectedInstallDisk, genOptions.TypedSpec().Value.InstallDisk)
		})
	}
}

func TestExtraKernelArgs(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Hour)
	defer cancel()

	sb := dynamicStateBuilder{m: map[resource.Namespace]state.CoreState{}}

	withRuntime(ctx, t, sb.Builder, func(ctx context.Context, st state.State, rt *runtime.Runtime, logger *zap.Logger) {
		controller := omnictrl.NewMachineConfigGenOptionsController()

		require.NoError(t, rt.RegisterQController(controller))
	}, func(ctx context.Context, st state.State, rt *runtime.Runtime, logger *zap.Logger) {
		const (
			installDisk        = "/dev/sdb"
			machineID          = "machine-1"
			schematicID        = "schematic-1"
			updatedSchematicID = "schematic-2"
		)

		var (
			args        = []string{"foo=bar"}
			updatedArgs = []string{"foo=bar", "bar=baz"}
		)

		cmtv := omni.NewClusterMachineTalosVersion(resources.DefaultNamespace, machineID)
		ms := omni.NewMachineStatus(resources.DefaultNamespace, machineID)

		cmtv.TypedSpec().Value.SchematicId = schematicID
		ms.TypedSpec().Value.SecurityState = &specs.SecurityState{}
		ms.TypedSpec().Value.Hardware = &specs.MachineStatusSpec_HardwareStatus{
			Blockdevices: []*specs.MachineStatusSpec_HardwareStatus_BlockDevice{
				{
					LinuxName:  installDisk,
					SystemDisk: true,
				},
			},
		}

		require.NoError(t, st.Create(ctx, cmtv))
		require.NoError(t, st.Create(ctx, ms))

		rtestutils.AssertResource(ctx, t, st, machineID, func(res *omni.MachineConfigGenOptions, assertion *assert.Assertions) {
			assertion.False(res.TypedSpec().Value.AlwaysIncludeKernelArgs)
			assertion.Equal(installDisk, res.TypedSpec().Value.InstallDisk)
			assertion.Equal(schematicID, res.TypedSpec().Value.InstallImage.SchematicId)
		})

		extraKernelArgs := omni.NewMachineExtraKernelArgs(resources.DefaultNamespace, machineID)
		extraKernelArgs.TypedSpec().Value.Args = args

		require.NoError(t, st.Create(ctx, extraKernelArgs))

		rtestutils.AssertResource(ctx, t, st, machineID, func(res *omni.MachineConfigGenOptions, assertion *assert.Assertions) {
			assertion.True(res.TypedSpec().Value.AlwaysIncludeKernelArgs)
			assertion.Equal(args, res.TypedSpec().Value.ExtraKernelArgs)
			assertion.Equal(schematicID, res.TypedSpec().Value.InstallImage.SchematicId)
		})

		logger.Info("update the schematic id, verify that schematic ID stays unchanged after reconciliation, because kernel args are not in sync yet (not observed on ClusterMachineConfigStatus)")

		// here, we want to assert that the schematic ID on the install image does not get updated until the kernel args are observed on the ClusterMachineConfigStatus, i.e., they are "synced".
		//
		// To be able to test that, we need a negative assertion. For that, we update the kernel args one more time, so that we can assert that a reconciliation was done,
		// but, as we expect, the schematic ID on the install image was not updated.
		_, err := safe.StateUpdateWithConflicts(ctx, st, cmtv.Metadata(), func(res *omni.ClusterMachineTalosVersion) error {
			res.TypedSpec().Value.SchematicId = updatedSchematicID

			return nil
		})
		require.NoError(t, err)

		_, err = safe.StateUpdateWithConflicts(ctx, st, extraKernelArgs.Metadata(), func(res *omni.MachineExtraKernelArgs) error {
			res.TypedSpec().Value.Args = updatedArgs

			return nil
		})
		require.NoError(t, err)

		rtestutils.AssertResource(ctx, t, st, machineID, func(res *omni.MachineConfigGenOptions, assertion *assert.Assertions) {
			assertion.Equal(updatedArgs, res.TypedSpec().Value.ExtraKernelArgs, "extra kernel args should get updated")
			assertion.True(res.TypedSpec().Value.AlwaysIncludeKernelArgs)
			assertion.Equal(schematicID, res.TypedSpec().Value.InstallImage.SchematicId, "schematic ID should not get updated until kernel args are in sync")
		})

		// get the kernel args "in sync" by creating the ClusterMachineConfigStatus with the expected args

		logger.Info("get kernel args in sync by creating ClusterMachineConfigStatus with the expected args")

		cmcs := omni.NewClusterMachineConfigStatus(resources.DefaultNamespace, machineID)
		cmcs.TypedSpec().Value.ExtraKernelArgs = updatedArgs

		require.NoError(t, st.Create(ctx, cmcs))

		rtestutils.AssertResource(ctx, t, st, machineID, func(res *omni.MachineConfigGenOptions, assertion *assert.Assertions) {
			assertion.True(res.TypedSpec().Value.AlwaysIncludeKernelArgs)
			assertion.Equal(updatedArgs, res.TypedSpec().Value.ExtraKernelArgs)
			assertion.Equal(updatedSchematicID, res.TypedSpec().Value.InstallImage.SchematicId)
		})

		logger.Info("destroy extra kernel args resource, expect it to be removed from the machine config gen options")

		rtestutils.Destroy[*omni.MachineExtraKernelArgs](ctx, t, st, []string{machineID})

		rtestutils.AssertResource(ctx, t, st, machineID, func(res *omni.MachineConfigGenOptions, assertion *assert.Assertions) {
			assertion.Empty(res.TypedSpec().Value.ExtraKernelArgs)
			assertion.True(res.TypedSpec().Value.AlwaysIncludeKernelArgs)
		})
	})
}
