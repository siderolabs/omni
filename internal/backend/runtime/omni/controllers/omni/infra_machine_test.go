// Copyright (c) 2025 Sidero Labs, Inc.
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

	link := siderolink.NewLink(resources.DefaultNamespace, "machine-1", &specs.SiderolinkSpec{})

	link.Metadata().Labels().Set(omni.LabelInfraProviderID, "bare-metal")
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
	machineStatus.TypedSpec().Value.SecurityState = &specs.SecurityState{}

	suite.Require().NoError(suite.state.Create(suite.ctx, machineStatus))

	assertResource(&suite.OmniSuite, infraMachineMD, func(r *infra.Machine, assertion *assert.Assertions) {
		assertion.Equal(specs.InfraMachineSpec_POWER_STATE_OFF, r.TypedSpec().Value.PreferredPowerState) // expect the default state of "OFF"
	})

	// accept the machine, set its preferred power state to on
	config := omni.NewInfraMachineConfig(resources.DefaultNamespace, "machine-1")
	config.TypedSpec().Value.AcceptanceStatus = specs.InfraMachineConfigSpec_ACCEPTED
	config.TypedSpec().Value.PowerState = specs.InfraMachineConfigSpec_POWER_STATE_ON
	config.TypedSpec().Value.ExtraKernelArgs = "foo=bar bar=baz"

	suite.Require().NoError(suite.state.Create(suite.ctx, config))

	assertResource(&suite.OmniSuite, infraMachineMD, func(r *infra.Machine, assertion *assert.Assertions) {
		assertion.Equal(specs.InfraMachineConfigSpec_ACCEPTED, r.TypedSpec().Value.AcceptanceStatus)
		assertion.Equal(specs.InfraMachineSpec_POWER_STATE_ON, r.TypedSpec().Value.PreferredPowerState)
		assertion.Equal("foo=bar bar=baz", r.TypedSpec().Value.ExtraKernelArgs)
	})

	// allocate the machine to a cluster
	clusterMachine := omni.NewClusterMachine(resources.DefaultNamespace, "machine-1")
	clusterMachine.Metadata().Labels().Set(omni.LabelCluster, "test-cluster")
	clusterMachine.Metadata().Labels().Set(omni.LabelMachineSet, "test-cluster-control-planes")
	clusterMachine.Metadata().Labels().Set(omni.LabelControlPlaneRole, "")

	suite.Require().NoError(suite.state.Create(suite.ctx, clusterMachine))

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

	// assert that the cluster machine has the correct extensions
	assertResource[*infra.Machine](&suite.OmniSuite, clusterMachine.Metadata(), func(r *infra.Machine, assertion *assert.Assertions) {
		assertion.ElementsMatch([]string{"foo", "bar"}, r.TypedSpec().Value.Extensions)
	})

	// deallocate the machine from the cluster
	rtestutils.Destroy[*omni.ClusterMachine](suite.ctx, suite.T(), suite.state, []string{clusterMachine.Metadata().ID()})

	// assert that cluster related fields are cleared, and a new wipe ID is generated
	assertResource[*infra.Machine](&suite.OmniSuite, infraMachineMD, func(r *infra.Machine, assertion *assert.Assertions) {
		assertion.False(r.Metadata().Labels().Matches(resource.LabelTerm{Key: omni.LabelCluster, Value: []string{"test-cluster"}, Op: resource.LabelOpEqual}))
		assertion.False(r.Metadata().Labels().Matches(resource.LabelTerm{Key: omni.LabelMachineSet, Value: []string{"test-cluster-control-planes"}, Op: resource.LabelOpEqual}))
		assertion.False(r.Metadata().Labels().Matches(resource.LabelTerm{Key: omni.LabelControlPlaneRole, Op: resource.LabelOpExists}))
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

	// reallocate the machine to a cluster
	suite.Require().NoError(suite.state.Create(suite.ctx, clusterMachine))

	// destroy the link
	rtestutils.Destroy[*siderolink.Link](suite.ctx, suite.T(), suite.state, []string{link.Metadata().ID()})

	// assert that infra.Machine is removed
	assertNoResource[*infra.Machine](&suite.OmniSuite, infraMachine)
}

func TestInfraMachineControllerSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(InfraMachineControllerSuite))
}
