// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package machineconfig_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/machineconfig"
)

func TestComputeHighPriorityConfigChanges(t *testing.T) {
	t.Parallel()

	const base = `apiVersion: v1alpha1
kind: RegistryAuthConfig
name: factory.example.org
username: user
password: pass
`

	authFor := func(host, username, password string) string {
		return "apiVersion: v1alpha1\nkind: RegistryAuthConfig\nname: " + host +
			"\nusername: " + username + "\npassword: " + password + "\n"
	}

	for _, tt := range []struct {
		name      string
		oldConfig string
		newConfig string
		expected  bool
	}{
		{
			name:      "no config at all",
			oldConfig: "",
			newConfig: "",
			expected:  false,
		},
		{
			name:      "identical auth",
			oldConfig: base,
			newConfig: base,
			expected:  false,
		},
		{
			name:      "auth added",
			oldConfig: "",
			newConfig: base,
			expected:  true,
		},
		{
			name:      "secondary factory auth added",
			oldConfig: base,
			newConfig: base + "---\n" + authFor("factory.secondary.example.org", "secondary-user", "secondary-pass"),
			expected:  true,
		},
		{
			name:      "auth modified",
			oldConfig: base,
			newConfig: authFor("factory.example.org", "user", "new-pass"),
			expected:  true,
		},
		{
			name:      "auth removed only",
			oldConfig: base,
			newConfig: "",
			expected:  false,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := machineconfig.ComputeHighPriorityConfigChanges([]byte(tt.oldConfig), []byte(tt.newConfig))
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}
