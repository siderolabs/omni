// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package machineset

import (
	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
)

// ReconcileStatus builds the machine set status from the resources.
//
//nolint:gocyclo,cyclop,gocognit
func ReconcileStatus(rc *ReconciliationContext, machineSetStatus *omni.MachineSetStatus) {
	spec := machineSetStatus.TypedSpec().Value

	// combined hash of all cluster machine config hashes
	spec.ConfigHash = rc.configHash

	machineSet := rc.GetMachineSet()
	machineSetNodes := rc.GetRunningMachineSetNodes()
	clusterMachines := rc.GetRunningClusterMachines()
	clusterMachineStatuses := rc.GetClusterMachineStatuses()

	helpers.CopyAllLabels(machineSet, machineSetStatus)

	spec.Phase = specs.MachineSetPhase_Running
	spec.Error = ""
	spec.Machines = &specs.Machines{}

	spec.Machines.Requested = uint32(len(machineSetNodes))

	// requested machines is max(manuallyAllocatedMachines, machineClassMachineCount)
	// if machine class allocation type is not static it falls back to the actual machineSetNodes count
	// then we first compare number of machine set nodes against the number of requested machines
	// if they match we compare the number of cluster machines against the number of machine set nodes
	machineAllocation := omni.GetMachineAllocation(machineSet)
	if machineAllocation != nil && machineAllocation.AllocationType == specs.MachineSetSpec_MachineAllocation_Static {
		spec.Machines.Requested = machineAllocation.MachineCount
	}

	spec.MachineAllocation = omni.GetMachineAllocation(machineSet)

	switch {
	case len(machineSetNodes) < int(spec.Machines.Requested):
		spec.Phase = specs.MachineSetPhase_ScalingUp
	case len(machineSetNodes) > int(spec.Machines.Requested):
		spec.Phase = specs.MachineSetPhase_ScalingDown
	case len(clusterMachineStatuses) < len(machineSetNodes):
		spec.Phase = specs.MachineSetPhase_ScalingUp
	case len(clusterMachines) > len(machineSetNodes):
		spec.Phase = specs.MachineSetPhase_ScalingDown
	}

	_, isControlPlane := machineSet.Metadata().Labels().Get(omni.LabelControlPlaneRole)

	if isControlPlane {
		// only allow config changes if no other operations are pending for the control planes
		spec.ConfigUpdatesAllowed = (len(rc.GetMachinesToCreate()) + len(rc.GetMachinesToTeardown()) + len(rc.GetMachinesToDestroy())) == 0
	} else {
		// always allow config updates for workers
		spec.ConfigUpdatesAllowed = true
	}

	spec.UpdateStrategy = machineSet.TypedSpec().Value.UpdateStrategy
	spec.UpdateStrategyConfig = machineSet.TypedSpec().Value.UpdateStrategyConfig

	if isControlPlane && len(machineSetNodes) == 0 && spec.Machines.Requested == 0 {
		spec.Phase = specs.MachineSetPhase_Failed
		spec.Error = "control plane machine set must have at least one node"
	}

	lockedCount := 0

	for _, clusterMachine := range clusterMachines {
		spec.Machines.Total++

		if clusterMachineStatus := clusterMachineStatuses[clusterMachine.Metadata().ID()]; clusterMachineStatus != nil {
			if _, updateLocked := clusterMachineStatus.Metadata().Labels().Get(omni.UpdateLocked); updateLocked {
				lockedCount++
			}

			if clusterMachineStatus.TypedSpec().Value.Stage == specs.ClusterMachineStatusSpec_RUNNING && clusterMachineStatus.TypedSpec().Value.Ready {
				spec.Machines.Healthy++
			}

			if _, ok := clusterMachineStatus.Metadata().Labels().Get(omni.MachineStatusLabelConnected); ok {
				spec.Machines.Connected++
			}
		}
	}

	spec.Ready = spec.Phase == specs.MachineSetPhase_Running
	spec.LockedUpdates = uint32(lockedCount)

	if !spec.Ready {
		return
	}

	switch {
	case len(rc.GetConfiguringMachines()) > 0:
		machineSetStatus.TypedSpec().Value.Phase = specs.MachineSetPhase_Reconfiguring

		spec.Ready = false
	case len(rc.GetUpgradingMachines()) > 0:
		machineSetStatus.TypedSpec().Value.Phase = specs.MachineSetPhase_Upgrading

		spec.Ready = false
	}

	for _, machineSetNode := range machineSetNodes {
		clusterMachine := clusterMachines[machineSetNode.Metadata().ID()]
		clusterMachineStatus := clusterMachineStatuses[machineSetNode.Metadata().ID()]

		if clusterMachine == nil || clusterMachineStatus == nil {
			spec.Ready = false
			spec.Phase = specs.MachineSetPhase_ScalingUp

			return
		}

		clusterMachineStatusSpec := clusterMachineStatus.TypedSpec().Value

		if clusterMachineStatusSpec.Stage != specs.ClusterMachineStatusSpec_RUNNING {
			spec.Ready = false

			return
		}

		if !clusterMachineStatusSpec.Ready {
			spec.Ready = false

			return
		}
	}

	if spec.Machines.Connected != spec.Machines.Total {
		spec.Ready = false
	}
}
