// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package grpc_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/pkg/kubeconfig"
	"github.com/siderolabs/omni/internal/backend/grpc"
	"github.com/siderolabs/omni/internal/pkg/config"
)

// TestServiceAccountKubeconfigValid makes sure the service account kubeconfig Omni generates passes the validation the client applies to it.
func TestServiceAccountKubeconfigValid(t *testing.T) {
	data, err := grpc.BuildServiceAccountKubeconfig(config.Default(), "cluster1", "sa1", "eyJhbGciOiJSUzI1NiJ9.e30.deadbeef")
	require.NoError(t, err)

	require.NoError(t, kubeconfig.Validate(data))
}
