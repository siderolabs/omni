// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"testing"

	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
)

// TestDiscoveryServiceEndpoint verifies that the discovery service endpoint is taken from the
// machine config Omni generated, applying Talos defaults, and that a disabled discovery yields an
// empty endpoint. This is the value Omni uses instead of reading the endpoint back from the node.
func TestDiscoveryServiceEndpoint(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name     string
		config   string
		expected string
	}{
		{
			name: "discovery enabled, custom endpoint",
			config: `cluster:
  discovery:
    enabled: true
    registries:
      service:
        endpoint: https://custom.discovery.example.com:8443`,
			expected: "https://custom.discovery.example.com:8443",
		},
		{
			name: "discovery enabled, embedded http endpoint",
			config: `cluster:
  discovery:
    enabled: true
    registries:
      service:
        endpoint: http://[fdae:41e4:649b:9303::1]:8090`,
			expected: "http://[fdae:41e4:649b:9303::1]:8090",
		},
		{
			name: "discovery enabled, no endpoint resolves to the Talos default",
			config: `cluster:
  discovery:
    enabled: true`,
			expected: constants.DefaultDiscoveryServiceEndpoint,
		},
		{
			name: "discovery disabled",
			config: `cluster:
  discovery:
    enabled: false
    registries:
      service:
        endpoint: https://custom.discovery.example.com:8443`,
			expected: "",
		},
		{
			name: "service registry disabled",
			config: `cluster:
  discovery:
    enabled: true
    registries:
      service:
        disabled: true
        endpoint: https://custom.discovery.example.com:8443`,
			expected: "",
		},
		{
			name: "no discovery section",
			config: `machine:
  network:
    kubespan:
      enabled: true`,
			expected: "",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg, err := configloader.NewFromBytes([]byte(tt.config))
			require.NoError(t, err)

			assert.Equal(t, tt.expected, omnictrl.DiscoveryServiceEndpoint(cfg))
		})
	}
}
