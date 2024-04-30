// Copyright (c) 2024 Sidero Labs, Inc.
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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/imagefactory"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
)

type SchematicConfigurationSuite struct {
	OmniSuite
}

func (suite *SchematicConfigurationSuite) TestReconcile() {
	ctx, cancel := context.WithTimeout(suite.ctx, time.Second*10)
	defer cancel()

	factory := imageFactoryMock{}
	suite.Require().NoError(factory.run())

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
	initialSchematic := "00000"
	clusterName := "cluster"
	machineSet := "machineset"

	cluster := omni.NewCluster(resources.DefaultNamespace, clusterName)
	cluster.TypedSpec().Value.TalosVersion = "1.7.0"

	suite.Require().NoError(suite.state.Create(ctx, cluster))

	machineStatus := omni.NewMachineStatus(resources.DefaultNamespace, machineName)
	machineStatus.TypedSpec().Value.Schematic = &specs.MachineStatusSpec_Schematic{
		InitialSchematic: initialSchematic,
	}
	machineStatus.TypedSpec().Value.InitialTalosVersion = "1.7.0"

	clusterMachine := omni.NewClusterMachine(resources.DefaultNamespace, machineName)
	clusterMachine.Metadata().Labels().Set(omni.LabelCluster, clusterName)
	clusterMachine.Metadata().Labels().Set(omni.LabelMachineSet, machineSet)

	suite.Require().NoError(suite.state.Create(ctx, machineStatus))
	suite.Require().NoError(suite.state.Create(ctx, clusterMachine))

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{machineName},
		func(schematicConfiguration *omni.SchematicConfiguration, assertion *assert.Assertions) {
			assertion.Equal(initialSchematic, schematicConfiguration.TypedSpec().Value.SchematicId)
		},
	)

	// set empty extensions list for the cluster
	extensionsConfiguration := omni.NewExtensionsConfiguration(resources.DefaultNamespace, "test")
	extensionsConfiguration.TypedSpec().Value.Extensions = []string{}
	extensionsConfiguration.Metadata().Labels().Set(omni.LabelCluster, clusterName)

	suite.Require().NoError(suite.state.Create(ctx, extensionsConfiguration))

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{machineName},
		func(schematicConfiguration *omni.SchematicConfiguration, assertion *assert.Assertions) {
			assertion.Equal("376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba", schematicConfiguration.TypedSpec().Value.SchematicId)
		},
	)

	// override extensions list for the machine set
	extensionsConfiguration = omni.NewExtensionsConfiguration(resources.DefaultNamespace, "machineset")
	extensionsConfiguration.TypedSpec().Value.Extensions = []string{
		"siderolabs/something",
	}
	extensionsConfiguration.Metadata().Labels().Set(omni.LabelCluster, clusterName)
	extensionsConfiguration.Metadata().Labels().Set(omni.LabelMachineSet, machineSet)

	suite.Require().NoError(suite.state.Create(ctx, extensionsConfiguration))

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{machineName},
		func(schematicConfiguration *omni.SchematicConfiguration, assertion *assert.Assertions) {
			assertion.Equal("df7c842f133b05c875f2139ea94b09eae3d425e00a95e6f9f54552f442d9f8c0", schematicConfiguration.TypedSpec().Value.SchematicId)
		},
	)

	// set overlay on the machine status
	_, err = safe.StateUpdateWithConflicts(ctx, suite.state, machineStatus.Metadata(), func(res *omni.MachineStatus) error {
		res.TypedSpec().Value.Schematic.Overlay = &specs.MachineStatusSpec_Schematic_Overlay{
			Name:  "rpi_generic",
			Image: "something",
		}

		return nil
	})

	suite.Require().NoError(err)

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{machineName},
		func(schematicConfiguration *omni.SchematicConfiguration, assertion *assert.Assertions) {
			assertion.Equal("f6a68c47512b4f3c50ccbd6d57873d2194dcac15f3a79d7703c05538a83429d7", schematicConfiguration.TypedSpec().Value.SchematicId)
		},
	)

	// override schematics on the machine level
	extensionsConfiguration = omni.NewExtensionsConfiguration(resources.DefaultNamespace, "zzzz")
	extensionsConfiguration.TypedSpec().Value.Extensions = []string{
		"siderolabs/something-else",
	}
	extensionsConfiguration.Metadata().Labels().Set(omni.LabelCluster, clusterName)
	extensionsConfiguration.Metadata().Labels().Set(omni.LabelClusterMachine, machineName)

	suite.Require().NoError(suite.state.Create(ctx, extensionsConfiguration))

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{machineName},
		func(schematicConfiguration *omni.SchematicConfiguration, assertion *assert.Assertions) {
			assertion.Equal("d7eb0c567b0b108e9b69ee0217c0fed99847175549b48d7b41ec6ef45d993965", schematicConfiguration.TypedSpec().Value.SchematicId)
		},
	)

	// update extensions
	_, err = safe.StateUpdateWithConflicts(ctx, suite.state, extensionsConfiguration.Metadata(), func(res *omni.ExtensionsConfiguration) error {
		res.TypedSpec().Value.Extensions = nil

		return nil
	})

	suite.Require().NoError(err)

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{machineName},
		func(schematicConfiguration *omni.SchematicConfiguration, assertion *assert.Assertions) {
			assertion.Equal("2611e4c1b6b8de906c9ad8f2248145d034ce8f657706407fe2f6a01086331a7d", schematicConfiguration.TypedSpec().Value.SchematicId)
		},
	)

	// reset everything to the default state, should revert back to the initial schematic

	rtestutils.DestroyAll[*omni.ExtensionsConfiguration](ctx, suite.T(), suite.state)

	suite.Require().NoError(err)

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{machineName},
		func(schematicConfiguration *omni.SchematicConfiguration, assertion *assert.Assertions) {
			assertion.Equal(initialSchematic, schematicConfiguration.TypedSpec().Value.SchematicId)
		},
	)

	_, err = safe.StateUpdateWithConflicts(ctx, suite.state, machineStatus.Metadata(), func(res *omni.MachineStatus) error {
		res.TypedSpec().Value.InitialTalosVersion = "1.5.0"

		return nil
	})

	suite.Require().NoError(err)

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{machineName},
		func(schematicConfiguration *omni.SchematicConfiguration, assertion *assert.Assertions) {
			assertion.Equal("35a502528a50b5c9d264a152545c4b02c2b82a2a5c8fd7398baa9fe78abfb1a2", schematicConfiguration.TypedSpec().Value.SchematicId)
		},
	)

	// set empty extensions list for the cluster, should keep the old schematic ID
	extensionsConfiguration.TypedSpec().Value.Extensions = []string{}
	extensionsConfiguration.Metadata().Labels().Set(omni.LabelCluster, clusterName)

	suite.Require().NoError(suite.state.Create(ctx, extensionsConfiguration))

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{machineName},
		func(schematicConfiguration *omni.SchematicConfiguration, assertion *assert.Assertions) {
			assertion.Equal("35a502528a50b5c9d264a152545c4b02c2b82a2a5c8fd7398baa9fe78abfb1a2", schematicConfiguration.TypedSpec().Value.SchematicId)
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
			assertion.Equal("35a502528a50b5c9d264a152545c4b02c2b82a2a5c8fd7398baa9fe78abfb1a2", schematicConfiguration.TypedSpec().Value.SchematicId)
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

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{machineName},
		func(schematicConfiguration *omni.SchematicConfiguration, assertion *assert.Assertions) {
			assertion.Equal("5fd4ef8a66795a9aba2520a2be1bb4fb64ef7405a775e40965cf6d7aa417665f", schematicConfiguration.TypedSpec().Value.SchematicId)
		},
	)
}

func TestSchematicConfigurationSuite(t *testing.T) {
	suite.Run(t, new(SchematicConfigurationSuite))
}
