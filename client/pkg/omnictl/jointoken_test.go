// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package omnictl //nolint:testpackage

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConstructJoinURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		baseURL  string
		tokenID  string
		expected string
	}{
		{
			name:     "no query params",
			baseURL:  "https://omni.siderolabs.io",
			tokenID:  "12345",
			expected: "https://omni.siderolabs.io?jointoken=12345",
		},
		{
			name:     "existing query params",
			baseURL:  "https://omni.siderolabs.io?foo=bar",
			tokenID:  "12345",
			expected: "https://omni.siderolabs.io?foo=bar&jointoken=12345",
		},
		{
			name:     "empty base URL",
			baseURL:  "",
			tokenID:  "12345",
			expected: "?jointoken=12345",
		},
		{
			name:     "trailing slash",
			baseURL:  "https://omni.siderolabs.io/",
			tokenID:  "12345",
			expected: "https://omni.siderolabs.io/?jointoken=12345",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			actual := constructJoinURL(tt.baseURL, tt.tokenID)
			assert.Equal(t, tt.expected, actual)
		})
	}
}
