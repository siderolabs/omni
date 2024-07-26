// Copyright (c) 2024 Sidero Labs, Inc.
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
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
)

type MachineSetNodeSuite struct {
	OmniSuite
	machinesOffset int
}

func newMachineClass(selectors ...string) *omni.MachineClass {
	id := uuid.New().String()

	cls := omni.NewMachineClass(resources.DefaultNamespace, id)
	cls.TypedSpec().Value.MatchLabels = selectors

	return cls
}

func (suite *MachineSetNodeSuite) createMachines(labels ...map[string]string) []*omni.MachineStatus {
	res := make([]*omni.MachineStatus, 0, len(labels))

	for i, l := range labels {
		id := fmt.Sprintf("machine%d", suite.machinesOffset+i)

		machine := omni.NewMachine(resources.DefaultNamespace, id)

		machineStatus := omni.NewMachineStatus(resources.DefaultNamespace, id)

		machineStatus.Metadata().Labels().Do(func(temp kvutils.TempKV) {
			for k, v := range l {
				temp.Set(k, v)
			}
		})

		machineStatus.TypedSpec().Value.TalosVersion = "v1.6.0"

		res = append(res, machineStatus)

		suite.Require().NoError(suite.state.Create(suite.ctx, machineStatus))
		suite.Require().NoError(suite.state.Create(suite.ctx, machine))
	}

	suite.machinesOffset += len(res)

	return res
}

func (suite *MachineSetNodeSuite) TestReconcile() {
	suite.startRuntime()

	ctx, cancel := context.WithTimeout(suite.ctx, time.Second*5)
	defer cancel()

	suite.Require().NoError(suite.runtime.RegisterController(&omnictrl.MachineSetNodeController{}))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewMachineSetStatusController()))

	machines := suite.createMachines(
		map[string]string{
			omni.MachineStatusLabelArch:            "amd64",
			omni.MachineStatusLabelAvailable:       "",
			omni.MachineStatusLabelConnected:       "",
			omni.MachineStatusLabelReportingEvents: "",
		},
		map[string]string{
			omni.MachineStatusLabelAvailable:       "",
			omni.MachineStatusLabelArch:            "amd64",
			omni.MachineStatusLabelConnected:       "",
			omni.MachineStatusLabelReportingEvents: "",
		},
		map[string]string{},
		map[string]string{
			omni.MachineStatusLabelCPU:             "AMD",
			omni.MachineStatusLabelAvailable:       "",
			"userlabel":                            "value",
			omni.MachineStatusLabelConnected:       "",
			omni.MachineStatusLabelReportingEvents: "",
		},
	)

	machineSet := omni.NewMachineSet(resources.DefaultNamespace, "auto")

	assertMachineSetNode := func(machine *omni.MachineStatus) {
		assertResource[*omni.MachineSetNode](
			&suite.OmniSuite,
			omni.NewMachineSetNode(
				resources.DefaultNamespace,
				machine.Metadata().ID(),
				machineSet).Metadata(),
			func(*omni.MachineSetNode, *assert.Assertions) {},
		)
	}

	assertNoMachineSetNode := func(machine *omni.MachineStatus) {
		assertNoResource[*omni.MachineSetNode](
			&suite.OmniSuite,
			omni.NewMachineSetNode(
				resources.DefaultNamespace,
				machine.Metadata().ID(),
				machineSet),
		)
	}

	cluster := omni.NewCluster(resources.DefaultNamespace, "cluster1")
	cluster.TypedSpec().Value.TalosVersion = "1.6.0"

	suite.Require().NoError(suite.state.Create(ctx, cluster))

	machineSet.Metadata().Labels().Set(omni.LabelCluster, cluster.Metadata().ID())
	machineSet.Metadata().Labels().Set(omni.LabelWorkerRole, "")

	machineClass := newMachineClass(fmt.Sprintf("%s==amd64", omni.MachineStatusLabelArch), "userlabel=value")

	machineSet.TypedSpec().Value.MachineClass = &specs.MachineSetSpec_MachineClass{
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
		ms.TypedSpec().Value.MachineClass.Name = machineClass.Metadata().ID()

		return nil
	})

	suite.Require().NoError(err)

	// no changes after updating machine set machine class
	assertNoMachineSetNode(machines[3])
	assertMachineSetNode(machines[0])

	_, err = safe.StateUpdateWithConflicts(ctx, suite.state, machineSet.Metadata(), func(ms *omni.MachineSet) error {
		ms.TypedSpec().Value.MachineClass.MachineCount = 0

		return nil
	})

	suite.Require().NoError(err)

	// scale down to 0 should remove machine set node
	assertNoMachineSetNode(machines[0])

	_, err = safe.StateUpdateWithConflicts(ctx, suite.state, machineSet.Metadata(), func(ms *omni.MachineSet) error {
		ms.TypedSpec().Value.MachineClass.MachineCount = 3

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
			omni.MachineStatusLabelReportingEvents: "",
		},
		map[string]string{
			omni.MachineStatusLabelCPU:             "AMD",
			omni.MachineStatusLabelConnected:       "",
			omni.MachineStatusLabelReportingEvents: "",
		},
		map[string]string{
			omni.MachineStatusLabelCPU:             "AMD",
			omni.MachineStatusLabelAvailable:       "",
			omni.MachineStatusLabelConnected:       "",
			omni.MachineStatusLabelReportingEvents: "",
		},
	)...)

	assertMachineSetNode(machines[4])
	assertMachineSetNode(machines[6])
	assertNoMachineSetNode(machines[5])

	rtestutils.Destroy[*omni.MachineStatus](ctx, suite.T(), suite.state, []string{machines[4].Metadata().ID()})
	rtestutils.Teardown[*omni.Machine](ctx, suite.T(), suite.state, []string{machines[4].Metadata().ID()})

	assertNoMachineSetNode(machines[4])
}

func (suite *MachineSetNodeSuite) TestRequiredExtraMachines() {
	ctx, cancel := context.WithTimeout(suite.ctx, time.Second*10)
	defer cancel()

	// prepare state and start runtime
	suite.Require().NoError(suite.runtime.RegisterController(&omnictrl.MachineSetNodeController{}))
	suite.Require().NoError(suite.runtime.RegisterController(omnictrl.NewMachineSetController()))

	cluster := omni.NewCluster(resources.DefaultNamespace, "test-cluster")
	cluster.TypedSpec().Value.TalosVersion = "1.7.5"

	suite.Require().NoError(suite.state.Create(ctx, cluster))

	machineClass := omni.NewMachineClass(resources.DefaultNamespace, "test-machine-class")
	machineClass.TypedSpec().Value.MatchLabels = []string{"foo=bar"}
	suite.Require().NoError(suite.state.Create(ctx, machineClass))

	machineSetMachineClassSpec := func() *specs.MachineSetSpec_MachineClass {
		return &specs.MachineSetSpec_MachineClass{
			Name:           machineClass.Metadata().ID(),
			AllocationType: specs.MachineSetSpec_MachineClass_Static,
			MachineCount:   3,
		}
	}

	machineSet := omni.NewMachineSet(resources.DefaultNamespace, "test-machine-set")
	machineSet.TypedSpec().Value.MachineClass = machineSetMachineClassSpec()

	machineSet.Metadata().Labels().Set(omni.LabelCluster, cluster.Metadata().ID())

	suite.Require().NoError(suite.state.Create(ctx, machineSet))

	suite.startRuntime()

	// assert that the MachineSetRequiredMachines resource is created with 3 required additional machines
	requiredMachines := omni.NewMachineSetRequiredMachines(resources.DefaultNamespace, machineSet.Metadata().ID())

	assertRequiredMachineCount := func(count int) {
		assertResource(&suite.OmniSuite, requiredMachines.Metadata(),
			func(r *omni.MachineSetRequiredMachines, assertion *assert.Assertions) {
				assertion.Equal(uint32(count), r.TypedSpec().Value.RequiredAdditionalMachines)
			})
	}

	assertRequiredMachineCount(3)
	assertResource(&suite.OmniSuite, requiredMachines.Metadata(),
		func(r *omni.MachineSetRequiredMachines, assertion *assert.Assertions) {
			machineClassNameLabel, ok := r.Metadata().Labels().Get(omni.LabelMachineClassName)

			assertion.True(ok, "MachineSetRequiredMachines resource should have a label %q", omni.LabelMachineClassName)
			assertion.Equal(machineClass.Metadata().ID(), machineClassNameLabel)
		})

	machineStatus := omni.NewMachineStatus(resources.DefaultNamespace, "test-machine-status")
	machineStatus.TypedSpec().Value.TalosVersion = "1.7.5"
	machineStatus.TypedSpec().Value.Hardware = &specs.MachineStatusSpec_HardwareStatus{
		Blockdevices: []*specs.MachineStatusSpec_HardwareStatus_BlockDevice{
			{
				SystemDisk: true,
				LinuxName:  "test-system-disk",
			},
		},
	}

	machineStatus.Metadata().Labels().Set(omni.MachineStatusLabelAvailable, "")
	machineStatus.Metadata().Labels().Set(omni.MachineStatusLabelConnected, "")
	machineStatus.Metadata().Labels().Set(omni.MachineStatusLabelReportingEvents, "")
	machineStatus.Metadata().Labels().Set("foo", "bar")

	suite.Require().NoError(suite.state.Create(ctx, machineStatus))
	assertRequiredMachineCount(2)

	// reduce the required machines to meet the number of available machines
	_, err := safe.StateUpdateWithConflicts(ctx, suite.state, machineSet.Metadata(), func(ms *omni.MachineSet) error {
		ms.TypedSpec().Value.MachineClass.MachineCount = 1

		return nil
	})
	suite.Require().NoError(err)

	assertRequiredMachineCount(0)

	// bring the pressure back for further testing
	_, err = safe.StateUpdateWithConflicts(ctx, suite.state, machineSet.Metadata(), func(ms *omni.MachineSet) error {
		ms.TypedSpec().Value.MachineClass = machineSetMachineClassSpec()

		return nil
	})
	suite.Require().NoError(err)

	assertRequiredMachineCount(2)

	// switch machine allocation mode to unlimited
	_, err = safe.StateUpdateWithConflicts(ctx, suite.state, machineSet.Metadata(), func(ms *omni.MachineSet) error {
		ms.TypedSpec().Value.MachineClass.AllocationType = specs.MachineSetSpec_MachineClass_Unlimited

		return nil
	})
	suite.Require().NoError(err)

	assertRequiredMachineCount(0)

	// remove the MachineClass spec from MachineSet
	_, err = safe.StateUpdateWithConflicts(ctx, suite.state, machineSet.Metadata(), func(ms *omni.MachineSet) error {
		ms.TypedSpec().Value.MachineClass = nil

		return nil
	})
	suite.Require().NoError(err)

	assertNoResource(&suite.OmniSuite, requiredMachines)

	// remove the MachineSet, assert that the MachineSetRequiredMachines resource is removed
	rtestutils.Destroy[*omni.MachineSet](ctx, suite.T(), suite.state, []string{machineSet.Metadata().ID()})
}

func TestSortFunction(t *testing.T) {
	machineStatuses := map[resource.ID]*omni.MachineStatus{}
	machineSetNodes := make([]*omni.MachineSetNode, 0, 10)

	for i := range 10 {
		id := strconv.Itoa(i)

		machineStatuses[id] = omni.NewMachineStatus(resources.DefaultNamespace, id)

		machineSetNode := omni.NewMachineSetNode(resources.DefaultNamespace, id, omni.NewMachineSet(resources.DefaultNamespace, "ms"))
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
