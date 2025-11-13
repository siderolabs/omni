// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package machineupgrade_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/siderolabs/image-factory/pkg/schematic"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	talosconstants "github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/imagefactory"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/machineupgrade"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils"
)

type mockImageFactoryClient struct{}

func (m *mockImageFactoryClient) EnsureSchematic(_ context.Context, inputSchematic schematic.Schematic) (imagefactory.EnsuredSchematic, error) {
	id, err := inputSchematic.ID()

	return imagefactory.EnsuredSchematic{
		FullID: id,
	}, err
}

func (m *mockImageFactoryClient) Host() string {
	return "mock-host"
}

type mockTalosClient struct {
	upgradeImages []string
}

func (m *mockTalosClient) Close() error {
	return nil
}

func (m *mockTalosClient) UpgradeWithOptions(_ context.Context, opt ...client.UpgradeOption) (*machine.UpgradeResponse, error) {
	var opts client.UpgradeOptions

	for _, o := range opt {
		o(&opts)
	}

	m.upgradeImages = append(m.upgradeImages, opts.Request.Image)

	return &machine.UpgradeResponse{}, nil
}

type mockTalosClientFactory struct {
	talosClient *mockTalosClient
}

func (m *mockTalosClientFactory) New(_ context.Context, _ string) (machineupgrade.TalosClient, error) {
	return m.talosClient, nil
}

func TestReconcile(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	imageFactoryClient := &mockImageFactoryClient{}
	talosClient := &mockTalosClient{}
	talosClientFactory := &mockTalosClientFactory{
		talosClient: talosClient,
	}

	testutils.WithRuntime(ctx, t, testutils.TestOptions{}, func(ctx context.Context, testContext testutils.TestContext) {
		ctrl := machineupgrade.NewStatusController(imageFactoryClient, talosClientFactory)

		require.NoError(t, testContext.Runtime.RegisterQController(ctrl))
	}, func(ctx context.Context, testContext testutils.TestContext) {
		const id = "test"

		ms := omni.NewMachineStatus(resources.DefaultNamespace, id)

		require.NoError(t, testContext.State.Create(ctx, ms))

		rtestutils.AssertResource(ctx, t, testContext.State, id, func(res *omni.MachineUpgradeStatus, assertion *assert.Assertions) {
			assertion.Equal(specs.MachineUpgradeStatusSpec_Unknown, res.TypedSpec().Value.Phase)
			assertion.Equal(res.TypedSpec().Value.Status, "schematic info is not available")
			assertion.Empty(res.TypedSpec().Value.Error)
		})

		initialSchematic := schematic.Schematic{
			Overlay: schematic.Overlay{
				Image:   "image",
				Name:    "name",
				Options: map[string]any{"key": "val"},
			},
			Customization: schematic.Customization{
				ExtraKernelArgs: []string{"arg1", "arg2"},
				Meta: []schematic.MetaValue{
					{Key: 1, Value: "value1"},
					{Key: 2, Value: "value2"},
				},
				SystemExtensions: schematic.SystemExtensions{
					OfficialExtensions: []string{"ext1", "ext2"},
				},
				SecureBoot: schematic.SecureBootCustomization{
					IncludeWellKnownCertificates: true,
				},
			},
		}

		currentSchematicID, err := initialSchematic.ID()
		require.NoError(t, err)

		currentSchematicRaw, err := initialSchematic.Marshal()
		require.NoError(t, err)

		kernelArgs := omni.NewKernelArgs(id)
		kernelArgs.TypedSpec().Value.Args = []string{"updated-arg1", "updated-arg2"}

		require.NoError(t, testContext.State.Create(ctx, kernelArgs))

		const talosVersion = "v1.11.3"

		_, err = safe.StateUpdateWithConflicts(ctx, testContext.State, ms.Metadata(), func(res *omni.MachineStatus) error {
			res.Metadata().Annotations().Set(omni.KernelArgsInitialized, "")

			res.TypedSpec().Value.Maintenance = true
			res.TypedSpec().Value.TalosVersion = talosVersion

			res.TypedSpec().Value.Hardware = &specs.MachineStatusSpec_HardwareStatus{
				Blockdevices: []*specs.MachineStatusSpec_HardwareStatus_BlockDevice{{LinuxName: "/dev/sda", SystemDisk: true}},
			}

			res.TypedSpec().Value.SecurityState = &specs.SecurityState{BootedWithUki: true}

			res.TypedSpec().Value.PlatformMetadata = &specs.MachineStatusSpec_PlatformMetadata{
				Platform: talosconstants.PlatformMetal,
			}

			res.TypedSpec().Value.Schematic = &specs.MachineStatusSpec_Schematic{
				Extensions:       initialSchematic.Customization.SystemExtensions.OfficialExtensions,
				InitialSchematic: currentSchematicID,
				Overlay: &specs.Overlay{
					Image: initialSchematic.Overlay.Image,
					Name:  initialSchematic.Overlay.Name,
				},
				KernelArgs: initialSchematic.Customization.ExtraKernelArgs,
				MetaValues: []*specs.MetaValue{
					{Key: 1, Value: "value1"},
					{Key: 2, Value: "value2"},
				},
				FullId: currentSchematicID,
				Raw:    string(currentSchematicRaw),
				InitialState: &specs.MachineStatusSpec_Schematic_InitialState{
					Extensions: initialSchematic.Customization.SystemExtensions.OfficialExtensions,
				},
			}

			return nil
		})
		require.NoError(t, err)

		rtestutils.AssertResource(ctx, t, testContext.State, id, func(res *omni.MachineUpgradeStatus, assertion *assert.Assertions) {
			assertion.Equal(specs.MachineUpgradeStatusSpec_Upgrading, res.TypedSpec().Value.Phase)
			assertion.Equal("Talos upgrade initiated", res.TypedSpec().Value.Status)
			assertion.Empty(res.TypedSpec().Value.Error)
			assertion.Equal(currentSchematicID, res.TypedSpec().Value.CurrentSchematicId)
			assertion.Equal(talosVersion, res.TypedSpec().Value.TalosVersion)
			assertion.Equal(talosVersion, res.TypedSpec().Value.CurrentTalosVersion)
		})

		updatedSchematic := initialSchematic
		updatedSchematic.Customization.ExtraKernelArgs = kernelArgs.TypedSpec().Value.Args

		updatedSchematicID, err := updatedSchematic.ID()
		require.NoError(t, err)

		updatedSchematicRaw, err := updatedSchematic.Marshal()
		require.NoError(t, err)

		// update MachineStatus to simulate upgrade completion

		_, err = safe.StateUpdateWithConflicts(ctx, testContext.State, ms.Metadata(), func(res *omni.MachineStatus) error {
			res.TypedSpec().Value.Schematic.FullId = updatedSchematicID
			res.TypedSpec().Value.Schematic.Raw = string(updatedSchematicRaw)

			return nil
		})
		require.NoError(t, err)

		rtestutils.AssertResource(ctx, t, testContext.State, id, func(res *omni.MachineUpgradeStatus, assertion *assert.Assertions) {
			assertion.Equal(specs.MachineUpgradeStatusSpec_UpToDate, res.TypedSpec().Value.Phase)
			assertion.Equal("machine is up to date", res.TypedSpec().Value.Status)
			assertion.Empty(res.TypedSpec().Value.Error)
			assertion.Equal(updatedSchematicID, res.TypedSpec().Value.CurrentSchematicId)
		})

		require.Len(t, talosClient.upgradeImages, 1)

		expectedInstallImage := fmt.Sprintf("mock-host/metal-installer/%s:v1.11.3", updatedSchematicID)

		assert.Equal(t, expectedInstallImage, talosClient.upgradeImages[0])

		// take it out of maintenance
		_, err = safe.StateUpdateWithConflicts(ctx, testContext.State, ms.Metadata(), func(res *omni.MachineStatus) error {
			res.TypedSpec().Value.Maintenance = false

			return nil
		})
		require.NoError(t, err)

		// assert that it was observed by the controller
		rtestutils.AssertResource(ctx, t, testContext.State, id, func(res *omni.MachineUpgradeStatus, assertion *assert.Assertions) {
			assertion.False(res.TypedSpec().Value.IsMaintenance)
		})

		// update the args to trigger a pending update
		_, err = safe.StateUpdateWithConflicts(ctx, testContext.State, kernelArgs.Metadata(), func(res *omni.KernelArgs) error {
			res.TypedSpec().Value.Args = []string{"final-arg1", "final-arg2"}

			return nil
		})
		require.NoError(t, err)

		rtestutils.AssertResource(ctx, t, testContext.State, id, func(res *omni.MachineUpgradeStatus, assertion *assert.Assertions) {
			assertion.Equal("not in maintenance mode", res.TypedSpec().Value.Status)
		})
	})
}
