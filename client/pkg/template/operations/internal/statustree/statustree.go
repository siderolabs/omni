// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package statustree contains helpers to render cluster status to the terminal.
package statustree

import (
	"cmp"
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/fatih/color"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// NodeWrapper wraps cluster status resources for tree rendering.
type NodeWrapper struct {
	resource.Resource
}

func (t NodeWrapper) String() string {
	switch node := t.Resource.(type) {
	case *omni.ClusterStatus:
		return fmt.Sprintf(
			"%s %q %s %s (%d/%d) (healthy/total)",
			color.YellowString("Cluster"),
			node.Metadata().ID(),
			clusterPhaseString(node.TypedSpec().Value.Phase),
			readyString(node.TypedSpec().Value.Ready),
			node.TypedSpec().Value.GetMachines().GetHealthy(),
			node.TypedSpec().Value.GetMachines().GetTotal(),
		)
	case *omni.KubernetesUpgradeStatus:
		return fmt.Sprintf(
			"%s %s %s",
			color.YellowString("Kubernetes Upgrade"),
			kubernetesUpgradePhaseString(node.TypedSpec().Value.Phase),
			node.TypedSpec().Value.Step,
		)
	case *omni.TalosUpgradeStatus:
		return fmt.Sprintf(
			"%s %s %s",
			color.YellowString("Talos Upgrade"),
			talosUpgradePhaseString(node.TypedSpec().Value.Phase),
			node.TypedSpec().Value.Step,
		)
	case *omni.MachineSetStatus:
		return fmt.Sprintf(
			"%s %q %s %s (%d/%d)",
			color.YellowString(machineSetName(node)),
			node.Metadata().ID(),
			machineSetPhaseString(node.TypedSpec().Value.Phase),
			readyString(node.TypedSpec().Value.Ready),
			node.TypedSpec().Value.GetMachines().GetHealthy(),
			node.TypedSpec().Value.GetMachines().GetTotal(),
		)
	case *omni.ClusterMachineStatus:
		return fmt.Sprintf(
			"%s %q %s%s%s%s%s",
			color.YellowString("Machine"),
			node.Metadata().ID(),
			clusterMachineStageString(node.TypedSpec().Value.Stage),
			clusterMachineReadyString(node),
			clusterMachineConnected(node),
			clusterMachineConfigOutdated(!node.TypedSpec().Value.ConfigUpToDate),
			clusterMachineConfigStatus(node.TypedSpec().Value.ConfigApplyStatus),
		)
	case *omni.ControlPlaneStatus:
		return fmt.Sprintf(
			"%s %s",
			color.YellowString("Status Checks"),
			controlPlaneStatusString(node),
		)
	case *omni.LoadBalancerStatus:
		return fmt.Sprintf(
			"%s %s",
			color.YellowString("Load Balancer"),
			readyString(node.TypedSpec().Value.Healthy),
		)
	default:
		return resource.String(t.Resource)
	}
}

// IsParentOf allows to find parent-child relationships between resources.
func (t NodeWrapper) IsParentOf(r resource.Resource) bool {
	switch node := t.Resource.(type) {
	case *omni.ClusterStatus:
		return r.Metadata().Type() == omni.MachineSetStatusType || r.Metadata().Type() == omni.KubernetesUpgradeStatusType || r.Metadata().Type() == omni.TalosUpgradeStatusType
	case *omni.MachineSetStatus:
		_, isControlPlane := node.Metadata().Labels().Get(omni.LabelControlPlaneRole)
		if isControlPlane && r.Metadata().Type() == omni.ControlPlaneStatusType {
			return true
		}

		if isControlPlane && r.Metadata().Type() == omni.LoadBalancerStatusType {
			return true
		}

		return r.Metadata().Type() == omni.ClusterMachineStatusType && r.Metadata().Labels().Matches(
			resource.LabelTerm{
				Key:   omni.LabelMachineSet,
				Op:    resource.LabelOpEqual,
				Value: []string{node.Metadata().ID()},
			})
	default:
		return false
	}
}

// resourceTypeOrder maps resource types to their order (integer) in the tree.
func resourceTypeOrder(resourceType resource.Type) int {
	switch resourceType {
	case omni.ClusterStatusType:
		return 0
	case omni.KubernetesUpgradeStatusType:
		return 1
	case omni.TalosUpgradeStatusType:
		return 2
	case omni.MachineSetStatusType:
		return 3
	case omni.LoadBalancerStatusType:
		return 4
	case omni.ControlPlaneStatusType:
		return 5
	case omni.ClusterMachineStatusType:
		return 6
	default:
		panic("unknown resource type " + resourceType)
	}
}

// Compare allows to sort resources in a tree on the same level.
func (t NodeWrapper) Compare(other NodeWrapper) int {
	l, r := t.Resource, other.Resource
	lType, rType := l.Metadata().Type(), r.Metadata().Type()
	lTypeOrder, rTypeOrder := resourceTypeOrder(lType), resourceTypeOrder(rType)

	if lTypeOrder != rTypeOrder {
		return lTypeOrder - rTypeOrder
	}

	// at this point lType == rType
	if lType == omni.MachineSetStatusType {
		// controlplane machine set goes first
		_, lIsControlPlane := l.Metadata().Labels().Get(omni.LabelControlPlaneRole)
		_, rIsControlPlane := r.Metadata().Labels().Get(omni.LabelControlPlaneRole)

		if lIsControlPlane {
			return -1
		}

		if rIsControlPlane {
			return 1
		}

		// now, default workers go first
		clusterName, _ := l.Metadata().Labels().Get(omni.LabelCluster)
		lIsDefaultWorkers := l.Metadata().ID() == omni.WorkersResourceID(clusterName)
		rIsDefaultWorkers := r.Metadata().ID() == omni.WorkersResourceID(clusterName)

		if lIsDefaultWorkers {
			return -1
		}

		if rIsDefaultWorkers {
			return 1
		}
	}

	// compare by name
	return cmp.Compare(l.Metadata().ID(), r.Metadata().ID())
}
