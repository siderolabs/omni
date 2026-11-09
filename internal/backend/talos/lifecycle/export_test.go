// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package lifecycle

import (
	"context"

	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// RelayProgressForTest exposes relayProgress to external tests.
func RelayProgressForTest(progress *machineapi.LifecycleServiceInstallProgress, operationLabel string, send func(string)) error {
	return relayProgress(progress, operationLabel, send)
}

// BuildInstallImageForTest exposes buildInstallImage to external tests.
func (m *Manager) BuildInstallImageForTest(
	ctx context.Context,
	machineID string,
	ms *omni.MachineStatus,
	version string,
	target *specs.MachineConfigGenOptionsSpec_InstallImage,
) (string, error) {
	return m.buildInstallImage(ctx, machineID, ms, version, target)
}

// AcquireForTest exposes acquire to external tests.
func (m *Manager) AcquireForTest(machineID string) bool {
	_, acquired := m.acquire(context.Background(), machineID, specs.MachineOperationSpec_PREPARING_IMAGE)

	return acquired
}

// StartForTest starts an operation on the machine and returns its progress callback.
func (m *Manager) StartForTest(machineID string) (progress func(string), started bool) {
	entry, acquired := m.acquire(context.Background(), machineID, specs.MachineOperationSpec_PREPARING_IMAGE)
	if !acquired {
		return nil, false
	}

	cfg := runConfig{progress: func(string) {}}

	m.track(context.Background(), machineID, entry, &cfg)

	return cfg.progress, true
}

// RunCordonAndDrainForTest exposes runCordonAndDrain to external tests.
func (m *Manager) RunCordonAndDrainForTest(ctx context.Context, clusterName, nodeName string, bestEffort bool, progress func(string)) error {
	return m.runCordonAndDrain(ctx, runConfig{
		cordonDrain: &nodeRef{clusterName: clusterName, nodeName: nodeName, bestEffort: bestEffort},
		progress:    progress,
		stage:       func(specs.MachineOperationSpec_Stage) {},
	})
}

// ReleaseForTest releases the current operation of the machine.
func (m *Manager) ReleaseForTest(machineID string, err error) {
	m.mu.Lock()
	entry := m.operations[machineID]
	m.mu.Unlock()

	m.release(context.Background(), machineID, entry, err)
}
