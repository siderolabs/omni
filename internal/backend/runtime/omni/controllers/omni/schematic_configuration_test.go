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
	"github.com/cosi-project/runtime/pkg/safe"
	talosconstants "github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/imagefactory"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
)

type SchematicConfigurationSuite struct {
	OmniSuite
}

//nolint:maintidx
func (suite *SchematicConfigurationSuite) TestReconcile() {
	ctx, cancel := context.WithTimeout(suite.ctx, time.Second*45)
	defer cancel()

	factory := imageFactoryMock{}
	suite.Require().NoError(factory.run(suite.ctx))

	factory.serve(ctx)

	defer func() {
		cancel()

		factory.eg.Wait() //nolint:errcheck
	}()

	imageFactoryClient, err := imagefactory.NewClient(suite.state, factory.address)
	suite.Require().NoError(err)

	suite.startRuntime()

	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewSchematicConfigurationController(imageFactoryClient)))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewMachineExtensionsController()))

	machineName := "machine1"
	clusterName := "cluster"
	machineSet := "machineset"

	const talosVersion = "1.7.0"

	cluster := omni.NewCluster(clusterName)
	cluster.TypedSpec().Value.TalosVersion = talosVersion

	suite.Require().NoError(suite.state.Create(ctx, cluster))

	machineStatus := omni.NewMachineStatus(machineName)
	machineStatus.Metadata().Annotations().Set(omni.KernelArgsInitialized, "")

	// customization:
	//   systemExtensions:
	//     officialExtensions:
	//       - siderolabs/hello-world-service
	expectedSchematic := "cf9b7aab9ed7c365d5384509b4d31c02fdaa06d2b3ac6cc0bc806f28130eff1f"

	machineStatus.TypedSpec().Value.TalosVersion = talosVersion
	machineStatus.TypedSpec().Value.Schematic = &specs.MachineStatusSpec_Schematic{
		Id:               "test-id",
		FullId:           "test-full-id",
		Extensions:       []string{"siderolabs/hello-world-service"},
		InitialSchematic: expectedSchematic,
		InitialState: &specs.MachineStatusSpec_Schematic_InitialState{
			Extensions: []string{"siderolabs/hello-world-service"},
		},
	}
	machineStatus.TypedSpec().Value.InitialTalosVersion = talosVersion
	machineStatus.TypedSpec().Value.SecurityState = &specs.SecurityState{
		BootedWithUki: true,
	}
	machineStatus.TypedSpec().Value.PlatformMetadata = &specs.MachineStatusSpec_PlatformMetadata{
		Platform: talosconstants.PlatformMetal,
	}

	clusterMachine := omni.NewClusterMachine(machineName)
	clusterMachine.Metadata().Labels().Set(omni.LabelCluster, clusterName)
	clusterMachine.Metadata().Labels().Set(omni.LabelMachineSet, machineSet)

	suite.Require().NoError(suite.state.Create(ctx, machineStatus))

	// a schematic should already be created with the current list of extensions, without requiring a cluster machine
	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{machineName},
		func(schematicConfiguration *omni.SchematicConfiguration, assertion *assert.Assertions) {
			_, hasClusterLabel := schematicConfiguration.Metadata().Labels().Get(omni.LabelCluster)
			assertion.False(hasClusterLabel)

			assertion.Equal(expectedSchematic, schematicConfiguration.TypedSpec().Value.SchematicId)
		},
	)

	suite.Require().NoError(suite.state.Create(ctx, clusterMachine))

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{machineName},
		func(schematicConfiguration *omni.SchematicConfiguration, assertion *assert.Assertions) {
			_, hasClusterLabel := schematicConfiguration.Metadata().Labels().Get(omni.LabelCluster)
			assertion.True(hasClusterLabel)

			assertion.Equal(expectedSchematic, schematicConfiguration.TypedSpec().Value.SchematicId)
		},
	)

	// set empty extensions list for the cluster
	extensionsConfiguration := omni.NewExtensionsConfiguration("test")
	extensionsConfiguration.TypedSpec().Value.Extensions = nil
	extensionsConfiguration.Metadata().Labels().Set(omni.LabelCluster, clusterName)

	suite.Require().NoError(suite.state.Create(ctx, extensionsConfiguration))

	// customization: {}
	expectedSchematic = "376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba"

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{machineName},
		func(schematicConfiguration *omni.SchematicConfiguration, assertion *assert.Assertions) {
			assertion.Equal(expectedSchematic, schematicConfiguration.TypedSpec().Value.SchematicId)
		},
	)

	// override extensions list for the machine set
	extensionsConfiguration = omni.NewExtensionsConfiguration("machineset")
	extensionsConfiguration.TypedSpec().Value.Extensions = []string{
		"siderolabs/something",
	}
	extensionsConfiguration.Metadata().Labels().Set(omni.LabelCluster, clusterName)
	extensionsConfiguration.Metadata().Labels().Set(omni.LabelMachineSet, machineSet)

	suite.Require().NoError(suite.state.Create(ctx, extensionsConfiguration))

	// customization:
	//   systemExtensions:
	//     officialExtensions:
	//       - siderolabs/something
	expectedSchematic = "df7c842f133b05c875f2139ea94b09eae3d425e00a95e6f9f54552f442d9f8c0"

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{machineName},
		func(schematicConfiguration *omni.SchematicConfiguration, assertion *assert.Assertions) {
			assertion.Equal(expectedSchematic, schematicConfiguration.TypedSpec().Value.SchematicId)
		},
	)

	// set overlay on the machine status
	_, err = safe.StateUpdateWithConflicts(ctx, suite.state, machineStatus.Metadata(), func(res *omni.MachineStatus) error {
		res.TypedSpec().Value.Schematic.Overlay = &specs.Overlay{
			Name:  "rpi_generic",
			Image: "something",
		}

		return nil
	})

	suite.Require().NoError(err)

	// overlay:
	//   image: something
	//   name: rpi_generic
	// customization:
	//   systemExtensions:
	//     officialExtensions:
	//       - siderolabs/something
	expectedSchematic = "f6a68c47512b4f3c50ccbd6d57873d2194dcac15f3a79d7703c05538a83429d7"

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{machineName},
		func(schematicConfiguration *omni.SchematicConfiguration, assertion *assert.Assertions) {
			assertion.Equal(expectedSchematic, schematicConfiguration.TypedSpec().Value.SchematicId)
		},
	)

	// override schematics on the machine level
	extensionsConfiguration = omni.NewExtensionsConfiguration("zzzz")
	extensionsConfiguration.TypedSpec().Value.Extensions = []string{
		"siderolabs/something-else",
	}
	extensionsConfiguration.Metadata().Labels().Set(omni.LabelCluster, clusterName)
	extensionsConfiguration.Metadata().Labels().Set(omni.LabelClusterMachine, machineName)

	suite.Require().NoError(suite.state.Create(ctx, extensionsConfiguration))

	// overlay:
	//   image: something
	//   name: rpi_generic
	// customization:
	//   systemExtensions:
	//     officialExtensions:
	//       - siderolabs/something-else
	expectedSchematic = "d7eb0c567b0b108e9b69ee0217c0fed99847175549b48d7b41ec6ef45d993965"

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{machineName},
		func(schematicConfiguration *omni.SchematicConfiguration, assertion *assert.Assertions) {
			assertion.Equal(expectedSchematic, schematicConfiguration.TypedSpec().Value.SchematicId)
		},
	)

	// update extensions
	_, err = safe.StateUpdateWithConflicts(ctx, suite.state, extensionsConfiguration.Metadata(), func(res *omni.ExtensionsConfiguration) error {
		res.TypedSpec().Value.Extensions = nil

		return nil
	})

	suite.Require().NoError(err)

	// overlay:
	//   image: something
	//   name: rpi_generic
	// customization: {}
	expectedSchematic = "2611e4c1b6b8de906c9ad8f2248145d034ce8f657706407fe2f6a01086331a7d"

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{machineName}, func(schematicConfiguration *omni.SchematicConfiguration, assertion *assert.Assertions) {
		assertion.Equal(expectedSchematic, schematicConfiguration.TypedSpec().Value.SchematicId)
	},
	)

	// reset everything to the default state, should revert back to the initial set of extensions

	rtestutils.DestroyAll[*omni.ExtensionsConfiguration](ctx, suite.T(), suite.state)

	suite.Require().NoError(err)

	// overlay:
	//   image: something
	//   name: rpi_generic
	// customization:
	//   systemExtensions:
	//     officialExtensions:
	//       - siderolabs/something
	expectedSchematic = "8ac31bbb181769d0963b217bb48f92839059ce90bc9e8b08836892c0182f8cb8"

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{machineName}, func(schematicConfiguration *omni.SchematicConfiguration, assertion *assert.Assertions) {
		assertion.Equal(expectedSchematic, schematicConfiguration.TypedSpec().Value.SchematicId)
	},
	)

	_, err = safe.StateUpdateWithConflicts(ctx, suite.state, machineStatus.Metadata(), func(res *omni.MachineStatus) error {
		res.TypedSpec().Value.InitialTalosVersion = "1.5.0"

		return nil
	})

	suite.Require().NoError(err)

	// overlay:
	//   image: something
	//   name: rpi_generic
	// customization:
	//   systemExtensions:
	//     officialExtensions:
	//       - siderolabs/bnx2-bnx2x
	//       - siderolabs/intel-ice-firmware
	expectedSchematic = "35a502528a50b5c9d264a152545c4b02c2b82a2a5c8fd7398baa9fe78abfb1a2"

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{machineName},
		func(schematicConfiguration *omni.SchematicConfiguration, assertion *assert.Assertions) {
			assertion.Equal(expectedSchematic, schematicConfiguration.TypedSpec().Value.SchematicId)
		},
	)

	// set empty extensions list for the cluster, should keep the old schematic ID
	extensionsConfiguration.TypedSpec().Value.Extensions = []string{}
	extensionsConfiguration.Metadata().Labels().Set(omni.LabelCluster, clusterName)

	suite.Require().NoError(suite.state.Create(ctx, extensionsConfiguration))

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{machineName},
		func(schematicConfiguration *omni.SchematicConfiguration, assertion *assert.Assertions) {
			assertion.Equal(expectedSchematic, schematicConfiguration.TypedSpec().Value.SchematicId)
		},
	)

	// update extensions, should be still no-op as it's duplicate to what's selected by Omni
	_, err = safe.StateUpdateWithConflicts(ctx, suite.state, extensionsConfiguration.Metadata(), func(res *omni.ExtensionsConfiguration) error {
		res.TypedSpec().Value.Extensions = []string{"siderolabs/bnx2-bnx2x"}

		return nil
	})

	suite.Require().NoError(err)

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{machineName},
		func(schematicConfiguration *omni.SchematicConfiguration, assertion *assert.Assertions) {
			assertion.Equal(expectedSchematic, schematicConfiguration.TypedSpec().Value.SchematicId)
		},
	)

	// add an extra extension, schematic ID should change
	_, err = safe.StateUpdateWithConflicts(ctx, suite.state, extensionsConfiguration.Metadata(), func(res *omni.ExtensionsConfiguration) error {
		res.TypedSpec().Value.Extensions = []string{
			"siderolabs/bnx2-bnx2x",
			"siderolabs/x11",
		}

		return nil
	})

	suite.Require().NoError(err)

	// overlay:
	//   image: something
	//   name: rpi_generic
	// customization:
	//   systemExtensions:
	//     officialExtensions:
	//       - siderolabs/bnx2-bnx2x
	//       - siderolabs/intel-ice-firmware
	//       - siderolabs/x11
	expectedSchematic = "5fd4ef8a66795a9aba2520a2be1bb4fb64ef7405a775e40965cf6d7aa417665f"

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{machineName}, func(schematicConfiguration *omni.SchematicConfiguration, assertion *assert.Assertions) {
		assertion.Equal(expectedSchematic, schematicConfiguration.TypedSpec().Value.SchematicId)
	},
	)

	// create kernel args, should change the schematic ID

	kernelArgs := omni.NewKernelArgs(machineName)
	kernelArgs.TypedSpec().Value.Args = []string{"foo=bar", "baz=qux"}

	suite.Require().NoError(suite.state.Create(ctx, kernelArgs))

	// overlay:
	//   image: something
	//   name: rpi_generic
	// customization:
	//   extraKernelArgs:
	//     - foo=bar
	//     - baz=qux
	//   systemExtensions:
	//     officialExtensions:
	//       - siderolabs/bnx2-bnx2x
	//       - siderolabs/intel-ice-firmware
	//       - siderolabs/x11
	expectedSchematic = "17b419c0d747bbd2399e2d06d16def170636569e9116e3e015b5be0015dd82c7"

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{machineName}, func(schematicConfiguration *omni.SchematicConfiguration, assertion *assert.Assertions) {
		assertion.Equal(expectedSchematic, schematicConfiguration.TypedSpec().Value.SchematicId)
	},
	)

	// set the UKI to false, the schematic should no more contain the kernel args (as updating them is not supported)

	_, err = safe.StateUpdateWithConflicts(ctx, suite.state, machineStatus.Metadata(), func(res *omni.MachineStatus) error {
		res.TypedSpec().Value.SecurityState.BootedWithUki = false

		return nil
	})
	suite.Require().NoError(err)

	// overlay:
	//   image: something
	//   name: rpi_generic
	// customization:
	//   systemExtensions:
	//     officialExtensions:
	//       - siderolabs/bnx2-bnx2x
	//       - siderolabs/intel-ice-firmware
	//       - siderolabs/x11
	expectedSchematic = "5fd4ef8a66795a9aba2520a2be1bb4fb64ef7405a775e40965cf6d7aa417665f"

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{machineName}, func(schematicConfiguration *omni.SchematicConfiguration, assertion *assert.Assertions) {
		assertion.Equal(expectedSchematic, schematicConfiguration.TypedSpec().Value.SchematicId)
	})

	// update the MachineStatus to simulate an actual change of the schematic (e.g., the schematic change caused an upgrade)
	_, err = safe.StateUpdateWithConflicts(ctx, suite.state, machineStatus.Metadata(), func(res *omni.MachineStatus) error {
		res.TypedSpec().Value.Schematic.Extensions = []string{
			"siderolabs/bnx2-bnx2x",
			"siderolabs/intel-ice-firmware",
			"siderolabs/x11",
		}

		return nil
	})
	suite.Require().NoError(err)

	// destroy the ClusterMachine
	rtestutils.Destroy[*omni.ClusterMachine](ctx, suite.T(), suite.state, []string{clusterMachine.Metadata().ID()})

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{machineName}, func(schematicConfiguration *omni.SchematicConfiguration, assertion *assert.Assertions) {
		_, hasClusterLabel := schematicConfiguration.Metadata().Labels().Get(omni.LabelCluster)
		assertion.False(hasClusterLabel)
	})

	// Change the extensions in the ExtensionsConfiguration: because the machine is no more allocated, it should be no-op, and the existing list of extensions should be preserved.
	_, err = safe.StateUpdateWithConflicts(ctx, suite.state, extensionsConfiguration.Metadata(), func(res *omni.ExtensionsConfiguration) error {
		res.TypedSpec().Value.Extensions = []string{
			"siderolabs/yet-another-extension",
		}

		return nil
	})
	suite.Require().NoError(err)

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{machineName}, func(schematicConfiguration *omni.SchematicConfiguration, assertion *assert.Assertions) {
		_, hasClusterLabel := schematicConfiguration.Metadata().Labels().Get(omni.LabelCluster)
		assertion.False(hasClusterLabel)

		assertion.Equal(expectedSchematic, schematicConfiguration.TypedSpec().Value.SchematicId)
	})
}

func TestSchematicConfigurationSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SchematicConfigurationSuite))
}
