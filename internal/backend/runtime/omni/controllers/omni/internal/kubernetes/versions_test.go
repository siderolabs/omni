// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package kubernetes_test

import (
	"context"
	"testing"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/kubernetes"
)

func TestCalculateUpgradeVersions(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name           string
		k8sVersions    []string
		talosVersion   string
		currentVersion string

		expected []string
	}{
		{
			name:           "to previous",
			k8sVersions:    []string{"1.25.0", "1.25.1", "1.25.2", "1.26.0", "1.26.1"},
			talosVersion:   "1.3.5",
			currentVersion: "1.26.1",
			expected:       []string{"1.26.0"},
		},
		{
			name:           "many versions",
			k8sVersions:    []string{"1.24.0", "1.25.0", "1.25.1", "1.25.2", "1.26.0", "1.26.1", "1.27.0"},
			talosVersion:   "1.3.5",
			currentVersion: "1.25.1",
			expected:       []string{"1.25.0", "1.25.2", "1.26.0", "1.26.1"},
		},
		{
			name:           "no compatible versions",
			k8sVersions:    []string{"1.22.1", "1.23.1", "1.24.1"},
			talosVersion:   "1.3.5",
			currentVersion: "1.22.1", // upgrade path only to 1.23, but Talos 1.3 doesn't support it
			expected:       nil,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()

			st := state.WrapCore(namespaced.NewState(inmem.Build))

			for _, version := range test.k8sVersions {
				v := omni.NewKubernetesVersion(resources.DefaultNamespace, version)
				v.TypedSpec().Value.Version = version

				require.NoError(t, st.Create(ctx, v))
			}

			versions, err := kubernetes.CalculateUpgradeVersions(ctx, st, test.currentVersion, test.talosVersion)
			require.NoError(t, err)

			assert.Equal(t, test.expected, versions)
		})
	}
}
