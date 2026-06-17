// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package lifecycle

import (
	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// BuildInstallImageForTest exposes buildInstallImage to external tests.
func (m *Manager) BuildInstallImageForTest(
	machineID string,
	ms *omni.MachineStatus,
	version string,
	target *specs.MachineConfigGenOptionsSpec_InstallImage,
) (string, error) {
	return m.buildInstallImage(machineID, ms, version, target)
}

// AcquireForTest exposes acquire to external tests.
func (m *Manager) AcquireForTest(machineID string) bool {
	return m.acquire(machineID)
}

// ReleaseForTest exposes release to external tests.
func (m *Manager) ReleaseForTest(machineID string) {
	m.release(machineID)
}
