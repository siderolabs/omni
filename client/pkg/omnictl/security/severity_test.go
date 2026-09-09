// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package security_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/pkg/omnictl/security"
)

func TestCountSeverities(t *testing.T) {
	for _, test := range []struct {
		name     string
		report   string
		expected security.SeverityCounts
	}{
		{
			name:     "no matches",
			report:   `{"matches": []}`,
			expected: security.SeverityCounts{},
		},
		{
			name: "one of each known severity",
			report: `{"matches": [
				{"vulnerability": {"severity": "Critical"}},
				{"vulnerability": {"severity": "High"}},
				{"vulnerability": {"severity": "Medium"}},
				{"vulnerability": {"severity": "Low"}}
			]}`,
			expected: security.SeverityCounts{Critical: 1, High: 1, Medium: 1, Low: 1},
		},
		{
			name: "unknown and negligible severities fall into Other",
			report: `{"matches": [
				{"vulnerability": {"severity": "Negligible"}},
				{"vulnerability": {"severity": "Unknown"}},
				{"vulnerability": {"severity": ""}}
			]}`,
			expected: security.SeverityCounts{Other: 3},
		},
		{
			name: "repeated severities accumulate",
			report: `{"matches": [
				{"vulnerability": {"severity": "Critical"}},
				{"vulnerability": {"severity": "Critical"}},
				{"vulnerability": {"severity": "High"}}
			]}`,
			expected: security.SeverityCounts{Critical: 2, High: 1},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			counts, err := security.CountSeverities([]byte(test.report))
			require.NoError(t, err)
			require.Equal(t, test.expected, counts)
		})
	}
}

func TestCountSeveritiesInvalidJSON(t *testing.T) {
	_, err := security.CountSeverities([]byte("not json"))
	require.Error(t, err)
}
