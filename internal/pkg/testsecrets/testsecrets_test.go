// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package testsecrets_test

import (
	"testing"

	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/internal/pkg/testsecrets"
)

func TestBundle(t *testing.T) {
	t.Parallel()

	for _, vc := range []*config.VersionContract{nil, config.TalosVersion1_2} {
		first, err := testsecrets.Bundle(vc)
		require.NoError(t, err)

		require.NotNil(t, first.Clock)
		require.NotNil(t, first.Cluster)
		require.NotNil(t, first.Secrets)
		require.NotNil(t, first.Certs)
		require.NotNil(t, first.Certs.OS)
		require.NotNil(t, first.Certs.K8sServiceAccount)

		// a second call returns an independent copy with the same content
		second, err := testsecrets.Bundle(vc)
		require.NoError(t, err)

		assert.NotSame(t, first, second)
		assert.Equal(t, first.Certs, second.Certs)
		assert.Equal(t, first.Cluster, second.Cluster)
	}
}
