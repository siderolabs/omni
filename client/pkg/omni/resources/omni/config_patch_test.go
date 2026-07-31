// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package omni_test

import (
	"fmt"
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
			expectedError: "error decoding document /v1alpha1/ (line 1): unknown keys found during decoding:\ncluster:\n    name: default\n",
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
		{
			// the forbidden fields live in the v1alpha1 document, which is not the first one here
			name: "forbidden fields behind another document",
			config: strings.TrimSpace(`
apiVersion: v1alpha1
kind: HostnameConfig
hostname: abcd
---
machine:
  token: aaa
`),
			expectedError: "1 error occurred:\n\t* overriding \"machine.token\" is not allowed in the config patch\n\n",
		},
		{
			name: "multi-doc patch of documents Omni does not own",
			config: strings.TrimSpace(`
machine:
  network:
    hostname: abcd
---
apiVersion: v1alpha1
kind: KubeletConfig
extraArgs:
  rotate-server-certificates: "true"
`),
		},
		{
			name: "os admin talos API access in its own document",
			config: strings.TrimSpace(`
apiVersion: v1alpha1
kind: KubeTalosAPIAccessConfig
allowedRoles:
  - os:reader
  - os:admin
allowedKubernetesNamespaces:
  - kube-system
`),
			expectedError: "1 error occurred:\n\t* element \"os:admin\" is not allowed in field \"allowedRoles\"\n\n",
		},
		{
			name: "talos API access document without the admin role",
			config: strings.TrimSpace(`
apiVersion: v1alpha1
kind: KubeTalosAPIAccessConfig
allowedRoles:
  - os:reader
allowedKubernetesNamespaces:
  - kube-system
`),
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

func TestForbiddenDocuments(t *testing.T) {
	for _, tt := range []struct {
		kind string
		body string
	}{
		{
			kind: "UnattendedInstallConfig",
			body: "installer:\n  image: example.com/installer:v1.14.0",
		},
		{
			kind: "DiscoveryIdentityConfig",
			body: "clusterID: YWFhCg==\nclusterSecret: YmJiCg==",
		},
		{
			kind: "KubeClusterConfig",
			body: "clusterName: talos-default\nendpoint: https://172.20.0.1:6443",
		},
		{
			kind: "KubeEtcdEncryptionConfig",
			body: "config:\n  resources:\n    - secrets",
		},
		{
			kind: "KubeAPIServerCAConfig",
			body: "issuingCA:\n  cert: YWFhCg==\n  key: YmJiCg==",
		},
		{
			kind: "KubeAggregatorCAConfig",
			body: "issuingCA:\n  cert: YWFhCg==\n  key: YmJiCg==",
		},
	} {
		t.Run(tt.kind, func(t *testing.T) {
			patch := fmt.Appendf(nil, "apiVersion: v1alpha1\nkind: %s\n%s\n", tt.kind, tt.body)

			require.EqualError(t, omni.ValidateConfigPatch(patch),
				fmt.Sprintf("1 error occurred:\n\t* the %q document is not allowed in the config patch\n\n", tt.kind))

			sanitized, err := omni.SanitizeConfigPatch(patch)
			require.NoError(t, err)
			require.Empty(t, sanitized)
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
			// nothing survives, and an empty document would be rejected by the Talos config loader
			name: "token",
			config: strings.TrimSpace(`
machine:
  token: aaa
`),
			sanitizedConfig: "",
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
		{
			name: "forbidden documents are dropped whole",
			config: strings.TrimSpace(`
apiVersion: v1alpha1
kind: UnattendedInstallConfig
installer:
  image: example.com/installer:v1.14.0
---
machine:
  network:
    hostname: abcd
---
apiVersion: v1alpha1
kind: KubeAPIServerCAConfig
issuingCA:
  cert: YWFhCg==
  key: YmJiCg==
`),
			sanitizedConfig: strings.TrimSpace(`
machine:
  network:
    hostname: abcd
`),
		},
		{
			name: "forbidden fields behind another document",
			config: strings.TrimSpace(`
apiVersion: v1alpha1
kind: HostnameConfig
hostname: abcd
---
machine:
  token: aaa
`),
			sanitizedConfig: strings.TrimSpace(`
apiVersion: v1alpha1
hostname: abcd
kind: HostnameConfig
`),
		},
		{
			name:            "nothing but forbidden documents",
			config:          "apiVersion: v1alpha1\nkind: UnattendedInstallConfig\ninstaller:\n  image: example.com/installer:v1.14.0",
			sanitizedConfig: "",
		},
		{
			name: "os admin talos API access in its own document",
			config: strings.TrimSpace(`
apiVersion: v1alpha1
kind: KubeTalosAPIAccessConfig
allowedRoles:
  - os:reader
  - os:admin
  - os:operator
allowedKubernetesNamespaces:
  - kube-system
`),
			sanitizedConfig: strings.TrimSpace(`
allowedKubernetesNamespaces:
  - kube-system
allowedRoles:
  - os:reader
  - os:operator
apiVersion: v1alpha1
kind: KubeTalosAPIAccessConfig
`),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			sanitizedPatch, err := omni.SanitizeConfigPatch([]byte(tt.config))
			require.NoError(t, err)
			require.Equal(t, tt.sanitizedConfig, strings.TrimSpace(string(sanitizedPatch)))

			// whatever survives has to be storable as a config patch again
			if len(sanitizedPatch) > 0 {
				require.NoError(t, omni.ValidateConfigPatch(sanitizedPatch))
			}
		})
	}
}
