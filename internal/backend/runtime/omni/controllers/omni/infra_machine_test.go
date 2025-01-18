// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"testing"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
)

type InfraMachineControllerSuite struct {
	OmniSuite
}

func (suite *InfraMachineControllerSuite) TestReconcile() {
	suite.startRuntime()

	installEventCh := make(chan resource.ID, 1)
	controller := omnictrl.NewInfraMachineController(installEventCh)

	suite.Require().NoError(suite.runtime.RegisterQController(controller))

	providerStatus := infra.NewProviderStatus("bare-metal")

	providerStatus.Metadata().Labels().Set(omni.LabelIsStaticInfraProvider, "")
	suite.Require().NoError(suite.state.Create(suite.ctx, providerStatus))

	link := siderolink.NewLink(resources.DefaultNamespace, "machine-1", nil)

	link.Metadata().Annotations().Set(omni.LabelInfraProviderID, "bare-metal")
	suite.Require().NoError(suite.state.Create(suite.ctx, link))

	infraMachine := infra.NewMachine("machine-1")
	infraMachineMD := infraMachine.Metadata()

	assertResource[*infra.Machine](&suite.OmniSuite, infraMachineMD, func(r *infra.Machine, assertion *assert.Assertions) {
		infraProviderID, ok := r.Metadata().Labels().Get(omni.LabelInfraProviderID)
		assertion.True(ok)
		assertion.Equal("bare-metal", infraProviderID)

		assertion.Equal(specs.InfraMachineSpec_POWER_STATE_ON, r.TypedSpec().Value.PreferredPowerState) // MachineStatus is not populated yet
		assertion.Equal(specs.InfraMachineConfigSpec_PENDING, r.TypedSpec().Value.AcceptanceStatus)
		assertion.Empty(r.TypedSpec().Value.ClusterTalosVersion)
		assertion.Empty(r.TypedSpec().Value.Extensions)
		assertion.Empty(r.TypedSpec().Value.WipeId)
		assertion.Zero(r.TypedSpec().Value.InstallEventId)
	})

	machineStatus := omni.NewMachineStatus(resources.DefaultNamespace, "machine-1")
	machineStatus.TypedSpec().Value.SecureBootStatus = &specs.SecureBootStatus{}

	suite.Require().NoError(suite.state.Create(suite.ctx, machineStatus))

	assertResource[*omni.MachineStatus](&suite.OmniSuite, machineStatus.Metadata(), func(r *omni.MachineStatus, assertion *assert.Assertions) {
		assertion.True(r.Metadata().Finalizers().Has(controller.Name()))
	})

	assertResource[*infra.Machine](&suite.OmniSuite, infraMachineMD, func(r *infra.Machine, assertion *assert.Assertions) {
		assertion.Equal(specs.InfraMachineSpec_POWER_STATE_OFF, r.TypedSpec().Value.PreferredPowerState) // expect the default state of "OFF"
	})

	// accept the machine, set its preferred power state to on
	config := omni.NewInfraMachineConfig(resources.DefaultNamespace, "machine-1")
	config.TypedSpec().Value.AcceptanceStatus = specs.InfraMachineConfigSpec_ACCEPTED
	config.TypedSpec().Value.PowerState = specs.InfraMachineConfigSpec_POWER_STATE_ON
	config.TypedSpec().Value.ExtraKernelArgs = "foo=bar bar=baz"

	suite.Require().NoError(suite.state.Create(suite.ctx, config))

	assertResource[*omni.InfraMachineConfig](&suite.OmniSuite, config.Metadata(), func(r *omni.InfraMachineConfig, assertion *assert.Assertions) {
		assertion.True(r.Metadata().Finalizers().Has(controller.Name()))
	})

	assertResource[*infra.Machine](&suite.OmniSuite, infraMachineMD, func(r *infra.Machine, assertion *assert.Assertions) {
		assertion.Equal(specs.InfraMachineConfigSpec_ACCEPTED, r.TypedSpec().Value.AcceptanceStatus)
		assertion.Equal(specs.InfraMachineSpec_POWER_STATE_ON, r.TypedSpec().Value.PreferredPowerState)
		assertion.Equal("foo=bar bar=baz", r.TypedSpec().Value.ExtraKernelArgs)
	})

	// allocate the machine to a cluster
	clusterMachine := omni.NewClusterMachine(resources.DefaultNamespace, "machine-1")

	suite.Require().NoError(suite.state.Create(suite.ctx, clusterMachine))

	// assert that the finalizer is added
	assertResource[*omni.ClusterMachine](&suite.OmniSuite, clusterMachine.Metadata(), func(r *omni.ClusterMachine, assertion *assert.Assertions) {
		assertion.True(r.Metadata().Finalizers().Has(controller.Name()))
	})

	// create schematic configuration
	schematicConfig := omni.NewSchematicConfiguration(resources.DefaultNamespace, "machine-1")

	schematicConfig.TypedSpec().Value.TalosVersion = "v42.0.0"

	suite.Require().NoError(suite.state.Create(suite.ctx, schematicConfig))

	// assert that the cluster machine has the correct talos version
	assertResource[*infra.Machine](&suite.OmniSuite, clusterMachine.Metadata(), func(r *infra.Machine, assertion *assert.Assertions) {
		assertion.Equal("v42.0.0", r.TypedSpec().Value.ClusterTalosVersion)
	})

	// add some extensions
	extensions := omni.NewMachineExtensions(resources.DefaultNamespace, "machine-1")

	extensions.TypedSpec().Value.Extensions = []string{"foo", "bar"}

	suite.Require().NoError(suite.state.Create(suite.ctx, extensions))

	assertResource[*omni.MachineExtensions](&suite.OmniSuite, extensions.Metadata(), func(r *omni.MachineExtensions, assertion *assert.Assertions) {
		assertion.True(r.Metadata().Finalizers().Has(controller.Name()))
	})

	// assert that the cluster machine has the correct extensions
	assertResource[*infra.Machine](&suite.OmniSuite, clusterMachine.Metadata(), func(r *infra.Machine, assertion *assert.Assertions) {
		assertion.ElementsMatch([]string{"foo", "bar"}, r.TypedSpec().Value.Extensions)
	})

	// deallocate the machine from the cluster
	rtestutils.Destroy[*omni.ClusterMachine](suite.ctx, suite.T(), suite.state, []string{clusterMachine.Metadata().ID()})

	// assert that the finalizer is removed, cluster related fields are cleared, and a new wipe ID is generated
	assertResource[*infra.Machine](&suite.OmniSuite, infraMachineMD, func(r *infra.Machine, assertion *assert.Assertions) {
		assertion.False(r.Metadata().Finalizers().Has(controller.Name()))

		assertion.Empty(r.TypedSpec().Value.ClusterTalosVersion)
		assertion.Empty(r.TypedSpec().Value.Extensions)
		assertion.NotEmpty(r.TypedSpec().Value.WipeId)
	})

	installEventCh <- infraMachineMD.ID()

	// assert that install id is incremented

	assertResource[*infra.Machine](&suite.OmniSuite, infraMachineMD, func(r *infra.Machine, assertion *assert.Assertions) {
		assertion.Equal(uint64(1), r.TypedSpec().Value.InstallEventId)
	})

	installEventCh <- infraMachineMD.ID()

	// assert that install id is incremented again

	assertResource[*infra.Machine](&suite.OmniSuite, infraMachineMD, func(r *infra.Machine, assertion *assert.Assertions) {
		assertion.Equal(uint64(2), r.TypedSpec().Value.InstallEventId)
	})

	// test finalizers

	// reallocate the machine to a cluster
	suite.Require().NoError(suite.state.Create(suite.ctx, clusterMachine))

	// assert that the finalizer is added
	assertResource[*omni.ClusterMachine](&suite.OmniSuite, clusterMachine.Metadata(), func(r *omni.ClusterMachine, assertion *assert.Assertions) {
		assertion.True(r.Metadata().Finalizers().Has(controller.Name()))
	})

	// destroy the link
	rtestutils.Destroy[*siderolink.Link](suite.ctx, suite.T(), suite.state, []string{link.Metadata().ID()})

	// assert that the finalizers are removed
	assertResource[*omni.ClusterMachine](&suite.OmniSuite, infraMachineMD, func(r *omni.ClusterMachine, assertion *assert.Assertions) {
		assertion.False(r.Metadata().Finalizers().Has(controller.Name()))
	})

	assertResource[*omni.InfraMachineConfig](&suite.OmniSuite, infraMachineMD, func(r *omni.InfraMachineConfig, assertion *assert.Assertions) {
		assertion.False(r.Metadata().Finalizers().Has(controller.Name()))
	})

	assertResource[*omni.MachineExtensions](&suite.OmniSuite, infraMachineMD, func(r *omni.MachineExtensions, assertion *assert.Assertions) {
		assertion.False(r.Metadata().Finalizers().Has(controller.Name()))
	})

	assertResource[*omni.MachineStatus](&suite.OmniSuite, infraMachineMD, func(r *omni.MachineStatus, assertion *assert.Assertions) {
		assertion.False(r.Metadata().Finalizers().Has(controller.Name()))
	})

	// assert that infra.Machine is removed
	assertNoResource[*infra.Machine](&suite.OmniSuite, infraMachine)
}

func TestInfraMachineControllerSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(InfraMachineControllerSuite))
}
