// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package models_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/template/internal/models"
)

// TestSystemExtensionsTranslate verifies that the system extensions list distinguishes between an
// unset list (no extensions configuration is produced, the machine keeps its current extensions) and
// an explicitly empty list (an extensions configuration with no extensions is produced, which clears
// the extensions instead of falling back to the initially discovered set).
func TestSystemExtensionsTranslate(t *testing.T) {
	for _, tt := range []struct { //nolint:govet
		name               string
		extensions         models.OptionalList
		expectConfig       bool
		expectedExtensions []string
	}{
		{
			name:         "unset",
			extensions:   models.OptionalList{},
			expectConfig: false,
		},
		{
			name:               "explicit empty",
			extensions:         models.NewOptionalList([]string{}),
			expectConfig:       true,
			expectedExtensions: []string{},
		},
		{
			name:               "populated",
			extensions:         models.NewOptionalList([]string{"siderolabs/hello-world-service"}),
			expectConfig:       true,
			expectedExtensions: []string{"siderolabs/hello-world-service"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cluster := models.Cluster{
				Meta:             models.Meta{Kind: models.KindCluster},
				Name:             "test-cluster",
				Kubernetes:       models.KubernetesCluster{Version: "v1.30.0"},
				Talos:            models.TalosCluster{Version: "v1.7.0"},
				SystemExtensions: tt.extensions,
			}

			resources, err := cluster.Translate(models.TranslateContext{ClusterName: "test-cluster"})
			require.NoError(t, err)

			var configs []*omni.ExtensionsConfiguration

			for _, res := range resources {
				if config, ok := res.(*omni.ExtensionsConfiguration); ok {
					configs = append(configs, config)
				}
			}

			if !tt.expectConfig {
				require.Empty(t, configs)

				return
			}

			require.Len(t, configs, 1)
			require.Equal(t, tt.expectedExtensions, configs[0].TypedSpec().Value.Extensions)

			clusterLabel, ok := configs[0].Metadata().Labels().Get(omni.LabelCluster)
			require.True(t, ok)
			require.Equal(t, "test-cluster", clusterLabel)
		})
	}
}
