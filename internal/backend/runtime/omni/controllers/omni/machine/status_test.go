// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package machine_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/machine"
)

func TestUpdateConnectionStatus(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		cluster                string
		name                   string
		infraPowerState        specs.InfraMachineStatusSpec_MachinePowerState
		powerStage             specs.MachineStatusSnapshotSpec_PowerStage
		isManagedByStaticInfra bool
		hasSnapshot            bool
		connected              bool
		infraReadyToUse        bool
		hasInfraMachineStatus  bool
		wantConnected          bool
		wantDisconnected       bool
		wantReadyToUse         bool
		wantConnectedVal       bool
	}{
		{
			name:             "connected machine",
			connected:        true,
			wantConnected:    true,
			wantDisconnected: false,
			wantReadyToUse:   true,
			wantConnectedVal: true,
		},
		{
			name:             "disconnected plain machine",
			connected:        false,
			wantConnected:    false,
			wantDisconnected: true,
			wantReadyToUse:   false,
			wantConnectedVal: false,
		},
		{
			name:             "powered-off machine",
			connected:        false,
			hasSnapshot:      true,
			powerStage:       specs.MachineStatusSnapshotSpec_POWER_STAGE_POWERED_OFF,
			wantConnected:    false,
			wantDisconnected: false,
			wantReadyToUse:   false,
			wantConnectedVal: false,
		},
		{
			name:                   "BM machine with BMC control, unallocated, ready to use",
			connected:              false,
			isManagedByStaticInfra: true,
			hasInfraMachineStatus:  true,
			infraPowerState:        specs.InfraMachineStatusSpec_POWER_STATE_OFF,
			infraReadyToUse:        true,
			wantConnected:          false,
			wantDisconnected:       false,
			wantReadyToUse:         true,
			wantConnectedVal:       false,
		},
		{
			name:                   "BM machine with BMC control, allocated, ready to use",
			connected:              false,
			isManagedByStaticInfra: true,
			cluster:                "some-cluster",
			hasInfraMachineStatus:  true,
			infraPowerState:        specs.InfraMachineStatusSpec_POWER_STATE_OFF,
			infraReadyToUse:        true,
			wantConnected:          false,
			wantDisconnected:       false,
			wantReadyToUse:         false,
			wantConnectedVal:       false,
		},
		{
			name:                   "BM machine with BMC control, not ready to use",
			connected:              false,
			isManagedByStaticInfra: true,
			hasInfraMachineStatus:  true,
			infraPowerState:        specs.InfraMachineStatusSpec_POWER_STATE_OFF,
			infraReadyToUse:        false,
			wantConnected:          false,
			wantDisconnected:       false,
			wantReadyToUse:         false,
			wantConnectedVal:       false,
		},
		{
			name:                   "BM machine, power state unknown (no BMC control)",
			connected:              false,
			isManagedByStaticInfra: true,
			hasInfraMachineStatus:  true,
			infraPowerState:        specs.InfraMachineStatusSpec_POWER_STATE_UNKNOWN,
			infraReadyToUse:        true,
			wantConnected:          false,
			wantDisconnected:       true,
			wantReadyToUse:         false,
			wantConnectedVal:       false,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			const id = "test-machine"

			machineRes := omni.NewMachine(id)
			machineRes.TypedSpec().Value.Connected = tt.connected

			if tt.isManagedByStaticInfra {
				machineRes.Metadata().Labels().Set(omni.LabelIsManagedByStaticInfraProvider, "")
			}

			machineStatus := omni.NewMachineStatus(id)
			machineStatus.TypedSpec().Value.Cluster = tt.cluster

			var snapshot *omni.MachineStatusSnapshot

			if tt.hasSnapshot {
				snapshot = omni.NewMachineStatusSnapshot(id)
				snapshot.TypedSpec().Value.PowerStage = tt.powerStage
			}

			var infraMachineStatus *infra.MachineStatus

			if tt.hasInfraMachineStatus {
				infraMachineStatus = infra.NewMachineStatus(id)
				infraMachineStatus.TypedSpec().Value.PowerState = tt.infraPowerState
				infraMachineStatus.TypedSpec().Value.ReadyToUse = tt.infraReadyToUse
			}

			machine.UpdateConnectionStatus(machineStatus, machineRes, snapshot, infraMachineStatus)

			_, hasConnected := machineStatus.Metadata().Labels().Get(omni.MachineStatusLabelConnected)
			_, hasDisconnected := machineStatus.Metadata().Labels().Get(omni.MachineStatusLabelDisconnected)
			_, hasReadyToUse := machineStatus.Metadata().Labels().Get(omni.MachineStatusLabelReadyToUse)

			assert.Equal(t, tt.wantConnected, hasConnected, "Connected label")
			assert.Equal(t, tt.wantDisconnected, hasDisconnected, "Disconnected label")
			assert.Equal(t, tt.wantReadyToUse, hasReadyToUse, "ReadyToUse label")
			assert.Equal(t, tt.wantConnectedVal, machineStatus.TypedSpec().Value.Connected, "Connected spec field")
		})
	}
}

func TestUpdatePowerState(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name                   string
		powerStage             specs.MachineStatusSnapshotSpec_PowerStage
		infraPowerState        specs.InfraMachineStatusSpec_MachinePowerState
		wantPowerState         specs.MachineStatusSpec_PowerState
		connected              bool
		isManagedByStaticInfra bool
		hasSnapshot            bool
		hasInfraMachineStatus  bool
	}{
		{
			name:           "connected non-BM machine",
			connected:      true,
			wantPowerState: specs.MachineStatusSpec_POWER_STATE_ON,
		},
		{
			name:           "disconnected non-BM machine",
			connected:      false,
			wantPowerState: specs.MachineStatusSpec_POWER_STATE_UNSUPPORTED,
		},
		{
			name:           "powered-off non-BM machine (snapshot)",
			connected:      false,
			hasSnapshot:    true,
			powerStage:     specs.MachineStatusSnapshotSpec_POWER_STAGE_POWERED_OFF,
			wantPowerState: specs.MachineStatusSpec_POWER_STATE_OFF,
		},
		{
			name:                   "BM machine, power on",
			isManagedByStaticInfra: true,
			hasInfraMachineStatus:  true,
			infraPowerState:        specs.InfraMachineStatusSpec_POWER_STATE_ON,
			wantPowerState:         specs.MachineStatusSpec_POWER_STATE_ON,
		},
		{
			name:                   "BM machine, power off",
			isManagedByStaticInfra: true,
			hasInfraMachineStatus:  true,
			infraPowerState:        specs.InfraMachineStatusSpec_POWER_STATE_OFF,
			wantPowerState:         specs.MachineStatusSpec_POWER_STATE_OFF,
		},
		{
			name:                   "BM machine, power state unknown",
			isManagedByStaticInfra: true,
			hasInfraMachineStatus:  true,
			infraPowerState:        specs.InfraMachineStatusSpec_POWER_STATE_UNKNOWN,
			wantPowerState:         specs.MachineStatusSpec_POWER_STATE_UNKNOWN,
		},
		{
			name:                   "BM machine, no infra status",
			isManagedByStaticInfra: true,
			hasInfraMachineStatus:  false,
			wantPowerState:         specs.MachineStatusSpec_POWER_STATE_UNKNOWN,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			const id = "test-machine"

			machineRes := omni.NewMachine(id)
			machineRes.TypedSpec().Value.Connected = tt.connected

			if tt.isManagedByStaticInfra {
				machineRes.Metadata().Labels().Set(omni.LabelIsManagedByStaticInfraProvider, "")
			}

			machineStatus := omni.NewMachineStatus(id)

			var snapshot *omni.MachineStatusSnapshot

			if tt.hasSnapshot {
				snapshot = omni.NewMachineStatusSnapshot(id)
				snapshot.TypedSpec().Value.PowerStage = tt.powerStage
			}

			var infraMachineStatus *infra.MachineStatus

			if tt.hasInfraMachineStatus {
				infraMachineStatus = infra.NewMachineStatus(id)
				infraMachineStatus.TypedSpec().Value.PowerState = tt.infraPowerState
			}

			machine.UpdatePowerState(machineStatus, machineRes, snapshot, infraMachineStatus)

			assert.Equal(t, tt.wantPowerState, machineStatus.TypedSpec().Value.PowerState, "PowerState")
		})
	}
}
