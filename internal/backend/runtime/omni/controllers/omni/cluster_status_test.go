// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"context"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
)

type ClusterStatusSuite struct {
	OmniSuite
}

func (suite *ClusterStatusSuite) TestReconcile() {
	suite.startRuntime()

	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewClusterStatusController(false)))

	for _, tt := range []struct { //nolint:govet
		name             string
		cpMachineSet     *specs.MachineSetStatusSpec
		workerMachineSet *specs.MachineSetStatusSpec
		cpStatus         *specs.ControlPlaneStatusSpec
		lbStatus         *specs.LoadBalancerStatusSpec
		expected         *specs.ClusterStatusSpec
	}{
		{
			name: "no statuses",
			expected: &specs.ClusterStatusSpec{
				Available:          false,
				Phase:              specs.ClusterStatusSpec_UNKNOWN,
				Ready:              false,
				ControlplaneReady:  false,
				KubernetesAPIReady: false,
				Machines: &specs.Machines{
					Total:   0,
					Healthy: 0,
				},
			},
		},
		{
			name: "all healthy",
			cpMachineSet: &specs.MachineSetStatusSpec{
				Phase: specs.MachineSetPhase_Running,
				Ready: true,
				Machines: &specs.Machines{
					Total:   3,
					Healthy: 3,
				},
			},
			workerMachineSet: &specs.MachineSetStatusSpec{
				Phase: specs.MachineSetPhase_Running,
				Ready: true,
				Machines: &specs.Machines{
					Total:   2,
					Healthy: 2,
				},
			},
			cpStatus: &specs.ControlPlaneStatusSpec{
				Conditions: []*specs.ControlPlaneStatusSpec_Condition{
					{
						Type:   specs.ConditionType_Etcd,
						Reason: "",
						Status: specs.ControlPlaneStatusSpec_Condition_Ready,
					},
				},
			},
			lbStatus: &specs.LoadBalancerStatusSpec{
				Healthy: true,
			},
			expected: &specs.ClusterStatusSpec{
				Available:          true,
				Phase:              specs.ClusterStatusSpec_RUNNING,
				Ready:              true,
				ControlplaneReady:  true,
				KubernetesAPIReady: true,
				Machines: &specs.Machines{
					Total:   5,
					Healthy: 5,
				},
			},
		},
		{
			name: "cp not healthy",
			cpMachineSet: &specs.MachineSetStatusSpec{
				Phase: specs.MachineSetPhase_Running,
				Ready: true,
				Machines: &specs.Machines{
					Total:   3,
					Healthy: 3,
				},
			},
			workerMachineSet: &specs.MachineSetStatusSpec{
				Phase: specs.MachineSetPhase_Running,
				Ready: true,
				Machines: &specs.Machines{
					Total:   2,
					Healthy: 2,
				},
			},
			cpStatus: &specs.ControlPlaneStatusSpec{
				Conditions: []*specs.ControlPlaneStatusSpec_Condition{
					{
						Type:   specs.ConditionType_Etcd,
						Reason: "",
						Status: specs.ControlPlaneStatusSpec_Condition_NotReady,
					},
				},
			},
			lbStatus: &specs.LoadBalancerStatusSpec{
				Healthy: false,
			},
			expected: &specs.ClusterStatusSpec{
				Available:          true,
				Phase:              specs.ClusterStatusSpec_RUNNING,
				Ready:              true,
				ControlplaneReady:  false,
				KubernetesAPIReady: false,
				Machines: &specs.Machines{
					Total:   5,
					Healthy: 5,
				},
			},
		},
		{
			name: "scaling up",
			cpMachineSet: &specs.MachineSetStatusSpec{
				Phase: specs.MachineSetPhase_ScalingUp,
				Ready: false,
				Machines: &specs.Machines{
					Total:   2,
					Healthy: 1,
				},
			},
			cpStatus: &specs.ControlPlaneStatusSpec{},
			lbStatus: &specs.LoadBalancerStatusSpec{
				Healthy: true,
			},
			expected: &specs.ClusterStatusSpec{
				Available:          true,
				Phase:              specs.ClusterStatusSpec_SCALING_UP,
				Ready:              false,
				ControlplaneReady:  false,
				KubernetesAPIReady: true,
				Machines: &specs.Machines{
					Total:   2,
					Healthy: 1,
				},
			},
		},
	} {
		suite.Run(tt.name, func() {
			ctx, cancel := context.WithTimeout(suite.ctx, 5*time.Second)
			defer cancel()

			clusterName := tt.name
			cluster, _ := suite.createCluster(clusterName, 3, 0)

			if tt.cpMachineSet != nil {
				machineSetStatus := omni.NewMachineSetStatus(resources.DefaultNamespace, clusterName+"-cp")
				machineSetStatus.Metadata().Labels().Set(omni.LabelCluster, clusterName)
				machineSetStatus.Metadata().Labels().Set(omni.LabelControlPlaneRole, "")

				machineSetStatus.TypedSpec().Value = tt.cpMachineSet

				suite.Require().NoError(suite.state.Create(ctx, machineSetStatus))
			}

			if tt.workerMachineSet != nil {
				machineSetStatus := omni.NewMachineSetStatus(resources.DefaultNamespace, omni.WorkersResourceID(clusterName))
				machineSetStatus.Metadata().Labels().Set(omni.LabelCluster, clusterName)
				machineSetStatus.Metadata().Labels().Set(omni.LabelWorkerRole, "")

				machineSetStatus.TypedSpec().Value = tt.workerMachineSet

				suite.Require().NoError(suite.state.Create(ctx, machineSetStatus))
			}

			if tt.cpStatus != nil {
				cpStatus := omni.NewControlPlaneStatus(resources.DefaultNamespace, clusterName+"-cp")
				cpStatus.Metadata().Labels().Set(omni.LabelCluster, clusterName)
				cpStatus.TypedSpec().Value = tt.cpStatus

				suite.Require().NoError(suite.state.Create(ctx, cpStatus))
			}

			if tt.lbStatus != nil {
				lbStatus := omni.NewLoadBalancerStatus(resources.DefaultNamespace, clusterName)
				lbStatus.TypedSpec().Value = tt.lbStatus

				suite.Require().NoError(suite.state.Create(ctx, lbStatus))
			}

			rtestutils.AssertResources(ctx, suite.T(), suite.state, []resource.ID{clusterName},
				func(status *omni.ClusterStatus, assert *assert.Assertions) {
					assert.Equal(status.TypedSpec().Value.Available, tt.expected.Available)
					assert.Equal(status.TypedSpec().Value.Phase, tt.expected.Phase)
					assert.Equal(status.TypedSpec().Value.ControlplaneReady, tt.expected.ControlplaneReady)
					assert.Equal(status.TypedSpec().Value.KubernetesAPIReady, tt.expected.KubernetesAPIReady)
					assert.Equal(status.TypedSpec().Value.Machines, tt.expected.Machines)
				})

			suite.destroyCluster(cluster)

			rtestutils.AssertNoResource[*omni.ClusterStatus](ctx, suite.T(), suite.state, clusterName)
		})
	}
}

func TestClusterStatusSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(ClusterStatusSuite))
}
