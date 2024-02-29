// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package machineset

import (
	"crypto/sha256"
	"encoding/hex"
	"slices"

	"github.com/siderolabs/gen/maps"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
)

// ReconcileStatus builds the machine set status from the resources.
//
//nolint:gocyclo,cyclop
func ReconcileStatus(rc *ReconciliationContext, machineSetStatus *omni.MachineSetStatus) {
	spec := machineSetStatus.TypedSpec().Value

	configHashHasher := sha256.New()

	clusterMachineConfigStatuses := rc.GetClusterMachineConfigStatuses()
	ids := maps.Keys(clusterMachineConfigStatuses)

	slices.Sort(ids)

	for _, id := range ids {
		configStatus := clusterMachineConfigStatuses[id]

		configHashHasher.Write([]byte(configStatus.TypedSpec().Value.ClusterMachineConfigSha256))
	}

	// combined hash of all cluster machine config hashes
	spec.ConfigHash = hex.EncodeToString(configHashHasher.Sum(nil))

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
	machineClass := machineSet.TypedSpec().Value.MachineClass
	if machineClass != nil && machineClass.AllocationType == specs.MachineSetSpec_MachineClass_Static {
		spec.Machines.Requested = machineClass.MachineCount
	}

	spec.MachineClass = machineClass

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

	if isControlPlane && len(machineSetNodes) == 0 {
		spec.Phase = specs.MachineSetPhase_Failed
		spec.Error = "control plane machine set must have at least one node"
	}

	for _, clusterMachine := range clusterMachines {
		spec.Machines.Total++

		if clusterMachineStatus := clusterMachineStatuses[clusterMachine.Metadata().ID()]; clusterMachineStatus != nil {
			if clusterMachineStatus.TypedSpec().Value.Stage == specs.ClusterMachineStatusSpec_RUNNING && clusterMachineStatus.TypedSpec().Value.Ready {
				spec.Machines.Healthy++
			}

			if _, ok := clusterMachineStatus.Metadata().Labels().Get(omni.MachineStatusLabelConnected); ok {
				spec.Machines.Connected++
			}
		}
	}

	spec.Ready = spec.Phase == specs.MachineSetPhase_Running

	if !spec.Ready {
		return
	}

	if len(rc.GetMachinesToUpdate()) > 0 || len(rc.GetOutdatedMachines()) > 0 {
		machineSetStatus.TypedSpec().Value.Phase = specs.MachineSetPhase_Reconfiguring

		spec.Ready = false

		return
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
