// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package statustree_test

import (
	"os"
	"testing"

	"github.com/fatih/color"
	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/template/operations/internal/statustree"
)

func TestMain(m *testing.M) {
	color.NoColor = true

	os.Exit(m.Run())
}

func TestDryRunString(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name     string
		spec     *specs.DryRunStatusSpec
		expected string
	}{
		{
			name:     "passed",
			spec:     &specs.DryRunStatusSpec{Outcome: specs.DryRunStatusSpec_PASSED},
			expected: "Dry run: passed",
		},
		{
			name:     "passed with reboot",
			spec:     &specs.DryRunStatusSpec{Outcome: specs.DryRunStatusSpec_PASSED, RebootRequired: true},
			expected: "Dry run: passed, reboots on unlock",
		},
		{
			name: "failed",
			spec: &specs.DryRunStatusSpec{
				Outcome: specs.DryRunStatusSpec_FAILED,
				Error:   "2 errors occurred:\n\t* disk selector is required\n\t* min size is greater than max size",
			},
			expected: "Dry run: failed\n2 errors occurred:\n\t* disk selector is required\n\t* min size is greater than max size",
		},
		{
			name:     "no pending changes",
			spec:     &specs.DryRunStatusSpec{Outcome: specs.DryRunStatusSpec_NO_PENDING_CHANGES},
			expected: "Dry run: no pending changes",
		},
		{
			name: "not checked",
			spec: &specs.DryRunStatusSpec{
				Outcome:          specs.DryRunStatusSpec_NOT_CHECKED,
				NotCheckedReason: specs.DryRunStatusSpec_MAINTENANCE,
			},
			expected: "Dry run: not checked (maintenance mode)",
		},
		{
			name: "disconnected",
			spec: &specs.DryRunStatusSpec{
				Outcome:          specs.DryRunStatusSpec_NOT_CHECKED,
				NotCheckedReason: specs.DryRunStatusSpec_DISCONNECTED,
			},
			expected: "Dry run: not checked (disconnected)",
		},
		{
			name: "try in flight",
			spec: &specs.DryRunStatusSpec{
				Outcome:          specs.DryRunStatusSpec_NOT_CHECKED,
				NotCheckedReason: specs.DryRunStatusSpec_TRY_IN_FLIGHT,
			},
			expected: "Dry run: not checked (config try in flight)",
		},
		{
			name: "not running",
			spec: &specs.DryRunStatusSpec{
				Outcome:          specs.DryRunStatusSpec_NOT_CHECKED,
				NotCheckedReason: specs.DryRunStatusSpec_NOT_RUNNING,
			},
			expected: "Dry run: not checked (not running)",
		},
		{
			name:     "outcome of a newer Omni",
			spec:     &specs.DryRunStatusSpec{Outcome: specs.DryRunStatusSpec_Outcome(99)},
			expected: "Dry run: unknown",
		},
		{
			name: "check error",
			spec: &specs.DryRunStatusSpec{
				Outcome:          specs.DryRunStatusSpec_NOT_CHECKED,
				NotCheckedReason: specs.DryRunStatusSpec_CHECK_ERROR,
				Error:            "context deadline exceeded",
			},
			expected: "Dry run: not checked (check failed)\ncontext deadline exceeded",
		},
		{
			name: "terminal escape in the error",
			spec: &specs.DryRunStatusSpec{
				Outcome: specs.DryRunStatusSpec_FAILED,
				Error:   "bad \x1b[2Jvalue",
			},
			expected: "Dry run: failed\nbad \\x1b[2Jvalue",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dryRun := omni.NewDryRunStatus("machine-1")
			dryRun.TypedSpec().Value = tt.spec

			assert.Equal(t, tt.expected, statustree.NodeWrapper{Resource: dryRun}.String())
		})
	}
}
