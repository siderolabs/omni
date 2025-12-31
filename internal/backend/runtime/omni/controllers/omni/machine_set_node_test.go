// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/kvutils"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/system"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
)

type MachineSetNodeSuite struct {
	OmniSuite

	machinesOffset int
}

func newMachineClass(selectors ...string) *omni.MachineClass {
	id := uuid.New().String()

	cls := omni.NewMachineClass(id)
	cls.TypedSpec().Value.MatchLabels = selectors

	return cls
}

func (suite *MachineSetNodeSuite) createMachines(labels ...map[string]string) []*omni.MachineStatus {
	res := make([]*omni.MachineStatus, 0, len(labels))

	ids := make([]string, 0, len(labels))

	for i, l := range labels {
		id := fmt.Sprintf("machine%d", suite.machinesOffset+i)

		machine := omni.NewMachine(id)

		machineStatus := omni.NewMachineStatus(id)

		machineStatus.Metadata().Labels().Do(func(temp kvutils.TempKV) {
			for k, v := range l {
				temp.Set(k, v)
			}
		})

		machineStatus.TypedSpec().Value.TalosVersion = "v1.6.0"
		machineStatus.Metadata().Labels().Set(omni.MachineStatusLabelTalosVersion, "v1.6.0")

		res = append(res, machineStatus)

		suite.Require().NoError(suite.state.Create(suite.ctx, machineStatus))
		suite.Require().NoError(suite.state.Create(suite.ctx, machine))

		ids = append(ids, machineStatus.Metadata().ID())
	}

	suite.machinesOffset += len(res)

	rtestutils.AssertResources(suite.ctx, suite.T(), suite.state, ids, func(*system.ResourceLabels[*omni.MachineStatus], *assert.Assertions) {})

	return res
}

func (suite *MachineSetNodeSuite) TestReconcile() {
	suite.startRuntime()

	ctx, cancel := context.WithTimeout(suite.ctx, time.Second*5)
	defer cancel()

	suite.Require().NoError(suite.runtime.RegisterQController(&omnictrl.MachineSetNodeController{}))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewMachineSetStatusController()))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewLabelsExtractorController[*omni.MachineStatus]()))

	machines := suite.createMachines(
		map[string]string{
			omni.MachineStatusLabelArch:            "amd64",
			omni.MachineStatusLabelAvailable:       "",
			omni.MachineStatusLabelConnected:       "",
			omni.MachineStatusLabelReadyToUse:      "",
			omni.MachineStatusLabelReportingEvents: "",
		},
		map[string]string{
			omni.MachineStatusLabelArch:            "amd64",
			omni.MachineStatusLabelAvailable:       "",
			omni.MachineStatusLabelConnected:       "",
			omni.MachineStatusLabelReadyToUse:      "",
			omni.MachineStatusLabelReportingEvents: "",
		},
		map[string]string{},
		map[string]string{
			omni.MachineStatusLabelCPU:             "AMD",
			omni.MachineStatusLabelAvailable:       "",
			"userlabel":                            "value",
			omni.MachineStatusLabelConnected:       "",
			omni.MachineStatusLabelReadyToUse:      "",
			omni.MachineStatusLabelReportingEvents: "",
		},
	)

	machineSet := omni.NewMachineSet("auto")

	assertMachineSetNode := func(machine *omni.MachineStatus) {
		rtestutils.AssertResources(
			ctx,
			suite.T(),
			suite.state,
			[]string{
				machine.Metadata().ID(),
			},
			func(n *omni.MachineSetNode, assert *assert.Assertions) {
				assert.Empty(n.Metadata().Owner())
			},
		)
	}

	assertNoMachineSetNode := func(machine *omni.MachineStatus) {
		rtestutils.AssertNoResource[*omni.MachineSetNode](
			ctx,
			suite.T(),
			suite.state,
			machine.Metadata().ID(),
		)
	}

	cluster := omni.NewCluster("cluster1")
	cluster.TypedSpec().Value.TalosVersion = "1.6.0"

	suite.Require().NoError(suite.state.Create(ctx, cluster))

	machineSet.Metadata().Labels().Set(omni.LabelCluster, cluster.Metadata().ID())
	machineSet.Metadata().Labels().Set(omni.LabelWorkerRole, "")

	machineClass := newMachineClass(fmt.Sprintf("%s==amd64", omni.MachineStatusLabelArch), "userlabel=value")

	machineSet.TypedSpec().Value.MachineAllocation = &specs.MachineSetSpec_MachineAllocation{
		Name:         machineClass.Metadata().ID(),
		MachineCount: 1,
	}

	suite.Require().NoError(suite.state.Create(ctx, machineClass))
	suite.Require().NoError(suite.state.Create(ctx, machineSet))

	assertMachineSetNode(machines[0])
	assertNoMachineSetNode(machines[1])
	assertNoMachineSetNode(machines[2])
	assertNoMachineSetNode(machines[3])

	machineClass = newMachineClass(fmt.Sprintf("%s==AMD", omni.MachineStatusLabelCPU))

	suite.Require().NoError(suite.state.Create(ctx, machineClass))

	_, err := safe.StateUpdateWithConflicts(ctx, suite.state, machineSet.Metadata(), func(ms *omni.MachineSet) error {
		ms.TypedSpec().Value.MachineAllocation.Name = machineClass.Metadata().ID()

		return nil
	})

	suite.Require().NoError(err)

	// no changes after updating machine set machine class
	// even though the node no longer matches the selector.
	// Omni should not automatically remove nodes.
	assertNoMachineSetNode(machines[3])
	assertMachineSetNode(machines[0])

	_, err = safe.StateUpdateWithConflicts(ctx, suite.state, machineSet.Metadata(), func(ms *omni.MachineSet) error {
		ms.TypedSpec().Value.MachineAllocation.MachineCount = 0

		return nil
	})

	suite.Require().NoError(err)

	// scale down to 0 should remove machine set node
	assertNoMachineSetNode(machines[0])

	_, err = safe.StateUpdateWithConflicts(ctx, suite.state, machineSet.Metadata(), func(ms *omni.MachineSet) error {
		ms.TypedSpec().Value.MachineAllocation.MachineCount = 3

		return nil
	})

	suite.Require().NoError(err)

	// scale back up to 3 after changing the machine class
	// should create a machine set node for the 3rd machine
	assertMachineSetNode(machines[3])
	assertNoMachineSetNode(machines[0])

	// add more machines and wait for the controller to scale up
	machines = append(machines, suite.createMachines(
		map[string]string{
			omni.MachineStatusLabelCPU:             "AMD",
			omni.MachineStatusLabelAvailable:       "",
			omni.MachineStatusLabelConnected:       "",
			omni.MachineStatusLabelReadyToUse:      "",
			omni.MachineStatusLabelReportingEvents: "",
		},
		map[string]string{
			omni.MachineStatusLabelCPU:             "AMD",
			omni.MachineStatusLabelConnected:       "",
			omni.MachineStatusLabelReadyToUse:      "",
			omni.MachineStatusLabelReportingEvents: "",
		},
		map[string]string{
			omni.MachineStatusLabelCPU:             "AMD",
			omni.MachineStatusLabelAvailable:       "",
			omni.MachineStatusLabelConnected:       "",
			omni.MachineStatusLabelReadyToUse:      "",
			omni.MachineStatusLabelReportingEvents: "",
		},
	)...)

	assertMachineSetNode(machines[4])
	assertMachineSetNode(machines[6])
	assertNoMachineSetNode(machines[5])

	// remove machine4 from Omni and see it being removed from the machine set
	rtestutils.Destroy[*omni.MachineStatus](ctx, suite.T(), suite.state, []string{machines[4].Metadata().ID()})
	rtestutils.Teardown[*omni.Machine](ctx, suite.T(), suite.state, []string{machines[4].Metadata().ID()})

	assertNoMachineSetNode(machines[4])

	// test machine request delete handling
	machineRequest := infra.NewMachineRequest("machine-request-1")

	suite.Require().NoError(suite.state.Create(ctx, machineRequest))

	machine := omni.NewMachine(machines[3].Metadata().ID()).Metadata()

	_, err = safe.StateUpdateWithConflicts(ctx, suite.state, machine,
		func(res *omni.Machine) error {
			res.Metadata().Labels().Set(omni.LabelMachineRequest, machineRequest.Metadata().ID())

			return nil
		},
	)

	suite.Require().NoError(err)

	rtestutils.Destroy[*infra.MachineRequest](ctx, suite.T(), suite.state, []string{machineRequest.Metadata().ID()})

	assertNoMachineSetNode(machines[3])

	rtestutils.Destroy[*omni.MachineSet](ctx, suite.T(), suite.state, []string{machineSet.Metadata().ID()})
}

func TestSortFunction(t *testing.T) {
	machineStatuses := map[resource.ID]*system.ResourceLabels[*omni.MachineStatus]{}
	machineSetNodes := make([]*omni.MachineSetNode, 0, 10)

	for i := range 10 {
		id := strconv.Itoa(i)

		machineStatuses[id] = system.NewResourceLabels[*omni.MachineStatus](id)

		machineSetNode := omni.NewMachineSetNode(id, omni.NewMachineSet("ms"))
		machineSetNode.Metadata().SetCreated(time.Now())

		machineSetNodes = append(machineSetNodes, machineSetNode)
	}

	slices.SortStableFunc(machineSetNodes, omnictrl.GetMachineSetNodeSortFunction(machineStatuses))

	require := require.New(t)

	for i := range len(machineSetNodes) - 1 {
		require.Equal(-1, machineSetNodes[i].Metadata().Created().Compare(machineSetNodes[i+1].Metadata().Created()))
	}

	machineStatuses["9"].Metadata().Labels().Set(omni.MachineStatusLabelDisconnected, "")

	slices.SortStableFunc(machineSetNodes, omnictrl.GetMachineSetNodeSortFunction(machineStatuses))

	machineStatuses["2"].Metadata().Labels().Set(omni.MachineStatusLabelDisconnected, "")

	slices.SortStableFunc(machineSetNodes, omnictrl.GetMachineSetNodeSortFunction(machineStatuses))

	require.Equal("2", machineSetNodes[0].Metadata().ID())
	require.Equal("9", machineSetNodes[1].Metadata().ID())
}

func TestMachineSetNodeSuite(t *testing.T) {
	suite.Run(t, new(MachineSetNodeSuite))
}
