// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package machine

import (
	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// UpdateConnectionStatus updates the connection-related labels and the Connected spec field on machineStatus
// based on the machine's current connectivity, power state, and infra provider information.
func UpdateConnectionStatus(machineStatus *omni.MachineStatus, machine *omni.Machine, machineStatusSnapshot *omni.MachineStatusSnapshot, infraMachineStatus *infra.MachineStatus) {
	connected := machine.TypedSpec().Value.Connected
	_, isManagedByStaticInfraProvider := machine.Metadata().Labels().Get(omni.LabelIsManagedByStaticInfraProvider)
	isPoweredOff := machineStatusSnapshot != nil && machineStatusSnapshot.TypedSpec().Value.GetPowerStage() == specs.MachineStatusSnapshotSpec_POWER_STAGE_POWERED_OFF
	canBePoweredOn := isManagedByStaticInfraProvider && infraMachineStatus != nil && infraMachineStatus.TypedSpec().Value.PowerState != specs.InfraMachineStatusSpec_POWER_STATE_UNKNOWN
	isUnallocated := machineStatus.TypedSpec().Value.Cluster == ""
	infraMachineIsReadyToUse := canBePoweredOn && infraMachineStatus.TypedSpec().Value.ReadyToUse

	machineStatus.TypedSpec().Value.Connected = connected

	if connected {
		machineStatus.Metadata().Labels().Set(omni.MachineStatusLabelConnected, "")
		machineStatus.Metadata().Labels().Delete(omni.MachineStatusLabelDisconnected)
		machineStatus.Metadata().Labels().Set(omni.MachineStatusLabelReadyToUse, "")

		return
	}

	// The logic for the disconnected machines

	machineStatus.Metadata().Labels().Delete(omni.MachineStatusLabelConnected)

	// If the machine is known to be powered off OR if we can power it on
	// (e.g., it is managed by the bare-metal infra provider,
	// and the provider has the BMC credentials of the machine),
	// we do not mark it as disconnected anymore, as the machine being not connected is expected.
	if isPoweredOff || canBePoweredOn {
		machineStatus.Metadata().Labels().Delete(omni.MachineStatusLabelDisconnected)
	} else {
		machineStatus.Metadata().Labels().Set(omni.MachineStatusLabelDisconnected, "")
	}

	// We mark the machine as "ready to use" from Omni's perspective, despite it being not connected, when all the conditions below are satisfied:
	// - The machine is unallocated, i.e., not yet part of a cluster
	// - It is managed by a static (e.g., bare-metal) provider
	// - The provider marked the machine as "ready to use" from its own perspective (no pending wipes etc.)
	// - The provider can actually power it on (e.g., using BMC) when needed (e.g., when it is added to a cluster)
	if isUnallocated && infraMachineIsReadyToUse {
		machineStatus.Metadata().Labels().Set(omni.MachineStatusLabelReadyToUse, "")
	} else {
		machineStatus.Metadata().Labels().Delete(omni.MachineStatusLabelReadyToUse)
	}
}

// UpdatePowerState updates the PowerState spec field on machineStatus.
// For machines managed by a static infra provider, the state comes from the infra provider.
// For other machines, it is derived from the snapshot power stage and connection state.
func UpdatePowerState(machineStatus *omni.MachineStatus, machine *omni.Machine, machineStatusSnapshot *omni.MachineStatusSnapshot, infraMachineStatus *infra.MachineStatus) {
	_, isManagedByStaticInfraProvider := machine.Metadata().Labels().Get(omni.LabelIsManagedByStaticInfraProvider)

	if isManagedByStaticInfraProvider {
		if infraMachineStatus == nil {
			machineStatus.TypedSpec().Value.PowerState = specs.MachineStatusSpec_POWER_STATE_UNKNOWN

			return
		}

		switch infraMachineStatus.TypedSpec().Value.PowerState {
		case specs.InfraMachineStatusSpec_POWER_STATE_UNKNOWN:
			machineStatus.TypedSpec().Value.PowerState = specs.MachineStatusSpec_POWER_STATE_UNKNOWN
		case specs.InfraMachineStatusSpec_POWER_STATE_ON:
			machineStatus.TypedSpec().Value.PowerState = specs.MachineStatusSpec_POWER_STATE_ON
		case specs.InfraMachineStatusSpec_POWER_STATE_OFF:
			machineStatus.TypedSpec().Value.PowerState = specs.MachineStatusSpec_POWER_STATE_OFF
		}

		return
	}

	isPoweredOff := machineStatusSnapshot != nil && machineStatusSnapshot.TypedSpec().Value.GetPowerStage() == specs.MachineStatusSnapshotSpec_POWER_STAGE_POWERED_OFF
	if isPoweredOff {
		machineStatus.TypedSpec().Value.PowerState = specs.MachineStatusSpec_POWER_STATE_OFF

		return
	}

	if machine.TypedSpec().Value.Connected {
		machineStatus.TypedSpec().Value.PowerState = specs.MachineStatusSpec_POWER_STATE_ON

		return
	}

	machineStatus.TypedSpec().Value.PowerState = specs.MachineStatusSpec_POWER_STATE_UNSUPPORTED
}
