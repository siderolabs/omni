// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package omni_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

func TestValidateConfigPatch(t *testing.T) {
	for _, tt := range []struct {
		name          string
		config        string
		expectedError string
	}{
		{
			name: "valid",
			config: strings.TrimSpace(`
machine:
  network:
    hostname: abcd
`),
		},
		{
			name: "token",
			config: strings.TrimSpace(`
machine:
  token: aaa
`),
			expectedError: "1 error occurred:\n\t* overriding \"machine.token\" is not allowed in the config patch\n\n",
		},
		{
			name: "several fields",
			config: strings.TrimSpace(`
machine:
  acceptedCAs:
    - crt: YWFhCg==
  token: bab
  ca:
    crt: YWFhCg==
cluster:
  acceptedCAs:
    - crt: YWFhCg==
    - crt: YmJiCg==
`),
			expectedError: `4 errors occurred:
	* overriding "cluster.acceptedCAs" is not allowed in the config patch
	* overriding "machine.token" is not allowed in the config patch
	* overriding "machine.ca" is not allowed in the config patch
	* overriding "machine.acceptedCAs" is not allowed in the config patch

`,
		},
		{
			name: "different configs",
			config: strings.TrimSpace(`
machine:
  ca:
    crt: YWFhCg==
cluster:
  name: default
`),
			expectedError: "unknown keys found during decoding:\ncluster:\n    name: default\n",
		},
		{
			name: "os admin talos API access",
			config: strings.TrimSpace(`
machine:
  features:
    kubernetesTalosAPIAccess:
      allowedRoles:
        - os:reader
        - os:admin
        - os:operator
`),
			expectedError: "1 error occurred:\n\t* element \"os:admin\" is not allowed in field \"machine.features.kubernetesTalosAPIAccess.allowedRoles\"\n\n",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := omni.ValidateConfigPatch([]byte(tt.config))
			if tt.expectedError != "" {
				require.Error(t, err, tt.expectedError)
				require.EqualError(t, err, tt.expectedError)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSanitizeConfigPatch(t *testing.T) {
	for _, tt := range []struct {
		name            string
		config          string
		sanitizedConfig string
	}{
		{
			name: "valid",
			config: strings.TrimSpace(`
machine:
  network:
    hostname: abcd
`),
			sanitizedConfig: strings.TrimSpace(`
machine:
  network:
    hostname: abcd
`),
		},
		{
			name: "token",
			config: strings.TrimSpace(`
machine:
  token: aaa
`),
			sanitizedConfig: strings.TrimSpace(`
{}
`),
		},
		{
			name: "several fields",
			config: strings.TrimSpace(`
machine:
  env:
    FOO: BAR
  acceptedCAs:
    - crt: YWFhCg==
  token: bab
  ca:
    crt: YWFhCg==
cluster:
  acceptedCAs:
    - crt: YWFhCg==
    - crt: YmJiCg==
  controlPlane:
    endpoint: https://172.20.0.1:6443
`),
			sanitizedConfig: strings.TrimSpace(`
cluster:
  controlPlane: {}
machine:
  env:
    FOO: BAR
`),
		},
		{
			name: "different configs",
			config: strings.TrimSpace(`
machine:
  ca:
    crt: YWFhCg==
cluster:
  network:
    dnsDomain: cluster.local
`),
			sanitizedConfig: strings.TrimSpace(`
cluster:
  network:
    dnsDomain: cluster.local
`),
		},
		{
			name: "os admin talos API access",
			config: strings.TrimSpace(`
machine:
  features:
    kubernetesTalosAPIAccess:
      allowedRoles:
        - os:reader
        - os:admin
        - os:operator
`),
			sanitizedConfig: strings.TrimSpace(`
machine:
  features:
    kubernetesTalosAPIAccess:
      allowedRoles:
        - os:reader
        - os:operator
`),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			sanitizedPatch, err := omni.SanitizeConfigPatch([]byte(tt.config))
			require.NoError(t, err)
			require.Equal(t, tt.sanitizedConfig, strings.TrimSpace(string(sanitizedPatch)))
		})
	}
}
