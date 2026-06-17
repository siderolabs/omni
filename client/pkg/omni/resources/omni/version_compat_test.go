// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package omni_test

import (
	"testing"

	"github.com/blang/semver/v4"
	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

func TestMachineCompatibleWithCluster(t *testing.T) {
	t.Parallel()

	mkMachine := func(version string, installed bool, agentMode bool) *omni.MachineStatus {
		ms := omni.NewMachineStatus("machine")
		ms.TypedSpec().Value.TalosVersion = version
		ms.Metadata().Labels().Set(omni.MachineStatusLabelTalosVersion, version)

		if installed {
			ms.Metadata().Labels().Set(omni.MachineStatusLabelInstalled, "")
		}

		if agentMode {
			ms.TypedSpec().Value.Schematic = &specs.MachineStatusSpec_Schematic{InAgentMode: true}
		}

		return ms
	}

	for _, tc := range []struct {
		name        string
		machine     *omni.MachineStatus
		cluster     string
		wantOK      bool
		wantInstall bool
	}{
		{
			name:    "installed same minor",
			machine: mkMachine("1.13.4", true, false),
			cluster: "1.13.4",
			wantOK:  true,
		},
		{
			name:    "installed older minor (upgrade path)",
			machine: mkMachine("1.12.5", true, false),
			cluster: "1.13.4",
			wantOK:  true,
		},
		{
			name:    "installed newer (downgrade rejected)",
			machine: mkMachine("1.14.0", true, false),
			cluster: "1.13.4",
			wantOK:  false,
		},
		{
			name:    "not-installed same minor",
			machine: mkMachine("1.13.4", false, false),
			cluster: "1.13.4",
			wantOK:  true,
		},
		{
			name:        "not-installed 1.13 maintenance to 1.13.4 (auto-install)",
			machine:     mkMachine("1.13.0", false, false),
			cluster:     "1.13.4",
			wantOK:      true,
			wantInstall: false, // same minor → no auto-install needed, normal config path
		},
		{
			name:        "not-installed 1.13 to higher 1.14 cluster (auto-install)",
			machine:     mkMachine("1.13.0", false, false),
			cluster:     "1.14.0",
			wantOK:      true,
			wantInstall: true,
		},
		{
			name:    "not-installed 1.12 minor mismatch (no install path)",
			machine: mkMachine("1.12.5", false, false),
			cluster: "1.13.4",
			wantOK:  false,
		},
		{
			name:    "agent mode always ok",
			machine: mkMachine("1.13.4", false, true),
			cluster: "1.14.0",
			wantOK:  true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			clusterVersion, err := semver.Parse(tc.cluster)
			assert.NoError(t, err)

			ok, willInstall, reason := omni.MachineCompatibleWithCluster(tc.machine, clusterVersion)
			assert.Equal(t, tc.wantOK, ok, "reason: %s", reason)
			assert.Equal(t, tc.wantInstall, willInstall, "reason: %s", reason)
		})
	}
}
