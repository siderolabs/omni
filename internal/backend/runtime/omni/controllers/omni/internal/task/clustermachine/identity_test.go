// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package clustermachine_test

import (
	"testing"

	"github.com/siderolabs/talos/pkg/machinery/resources/cluster"
	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/task/clustermachine"
)

func TestDiscoveryServiceEndpoints(t *testing.T) {
	for _, tt := range []struct {
		name     string
		spec     cluster.ConfigSpec
		expected []string
	}{
		{
			name:     "empty",
			expected: nil,
		},
		{
			name: "multiple service endpoints are all returned",
			spec: cluster.ConfigSpec{
				ServiceEndpoints: []cluster.ServiceEndpoint{
					{Name: "primary", Endpoint: "discovery.talos.dev:443"},
					{Name: "secondary", Endpoint: "discovery.example.org:443"},
				},
			},
			expected: []string{"https://discovery.talos.dev:443", "https://discovery.example.org:443"},
		},
		{
			name: "insecure service endpoint",
			spec: cluster.ConfigSpec{
				ServiceEndpoints: []cluster.ServiceEndpoint{
					{Name: "primary", Endpoint: "discovery.talos.dev:80", Insecure: true},
				},
			},
			expected: []string{"http://discovery.talos.dev:80"},
		},
		{
			name: "full URL endpoints are kept as-is",
			spec: cluster.ConfigSpec{
				ServiceEndpoints: []cluster.ServiceEndpoint{
					{Name: "primary", Endpoint: "https://discovery.talos.dev/"},
				},
			},
			expected: []string{"https://discovery.talos.dev/"},
		},
		{
			name: "legacy fields",
			spec: cluster.ConfigSpec{
				DiscoveryEnabled:       true,
				RegistryServiceEnabled: true,
				ServiceEndpoint:        "discovery.talos.dev:443",
			},
			expected: []string{"https://discovery.talos.dev:443"},
		},
		{
			name: "insecure legacy fields",
			spec: cluster.ConfigSpec{
				DiscoveryEnabled:        true,
				RegistryServiceEnabled:  true,
				ServiceEndpoint:         "discovery.talos.dev:80",
				ServiceEndpointInsecure: true,
			},
			expected: []string{"http://discovery.talos.dev:80"},
		},
		{
			name: "legacy fields with service registry disabled",
			spec: cluster.ConfigSpec{
				DiscoveryEnabled: true,
				ServiceEndpoint:  "discovery.talos.dev:443",
			},
			expected: nil,
		},
		{
			name: "empty service endpoint value falls back to legacy fields",
			spec: cluster.ConfigSpec{
				ServiceEndpoints: []cluster.ServiceEndpoint{
					{Name: "primary"},
				},
				DiscoveryEnabled:       true,
				RegistryServiceEnabled: true,
				ServiceEndpoint:        "legacy.talos.dev:443",
			},
			expected: []string{"https://legacy.talos.dev:443"},
		},
		{
			name: "service endpoints take precedence over legacy fields",
			spec: cluster.ConfigSpec{
				ServiceEndpoints: []cluster.ServiceEndpoint{
					{Name: "primary", Endpoint: "discovery.talos.dev:443"},
				},
				DiscoveryEnabled:       true,
				RegistryServiceEnabled: true,
				ServiceEndpoint:        "legacy.talos.dev:443",
			},
			expected: []string{"https://discovery.talos.dev:443"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			res := cluster.NewConfig(cluster.NamespaceName, cluster.ConfigID)
			*res.TypedSpec() = tt.spec

			assert.Equal(t, tt.expected, clustermachine.DiscoveryServiceEndpoints(res))
		})
	}
}
