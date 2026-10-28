// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package grpc_test

import (
	"testing"

	clientconfig "github.com/siderolabs/talos/pkg/machinery/client/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"

	ctlcfg "github.com/siderolabs/omni/client/pkg/omnictl/config"
	grpcomni "github.com/siderolabs/omni/internal/backend/grpc"
	"github.com/siderolabs/omni/internal/pkg/auth"
	"github.com/siderolabs/omni/internal/pkg/config"
)

func TestInstanceContextName(t *testing.T) {
	t.Parallel()

	name, id := "instance", "edd2822a-7834-4fe0-8172-cc5581f13a8d"

	assert.Equal(t, "instance", grpcomni.InstanceContextName(&config.Account{Name: &name, Id: &id}))
}

func TestGenerateConfig(t *testing.T) {
	t.Parallel()

	const (
		url             = "https://instance.omni.example.org"
		identity        = "user@example.com"
		instanceContext = "instance"
	)

	data, err := grpcomni.GenerateConfig(auth.CheckResult{Identity: identity}, url, instanceContext)
	require.NoError(t, err)

	var cfg ctlcfg.Config

	require.NoError(t, yaml.Unmarshal(data, &cfg))

	assert.Equal(t, instanceContext, cfg.Context)
	require.Len(t, cfg.Contexts, 1)
	require.Contains(t, cfg.Contexts, instanceContext)

	context := cfg.Contexts[instanceContext]

	assert.Equal(t, url, context.URL)
	assert.Equal(t, identity, context.Auth.SideroV1.Identity)
}

func TestGenerateTalosconfig(t *testing.T) {
	t.Parallel()

	const (
		url             = "https://instance.omni.example.org"
		identity        = "user@example.com"
		instanceContext = "instance"
	)

	for _, tt := range []struct {
		name                string
		clusterName         string
		expectedContextName string
	}{
		{
			// omnictl falls back to the talosctl keys, so the instance context must match the omniconfig one
			name:                "instance",
			expectedContextName: instanceContext,
		},
		{
			name:                "cluster",
			clusterName:         "cluster1",
			expectedContextName: instanceContext + "-cluster1",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			data, err := grpcomni.GenerateTalosconfig(instanceContext, url, tt.clusterName, identity)
			require.NoError(t, err)

			cfg, err := clientconfig.FromBytes(data)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedContextName, cfg.Context)
			require.Len(t, cfg.Contexts, 1)
			require.Contains(t, cfg.Contexts, tt.expectedContextName)

			context := cfg.Contexts[tt.expectedContextName]

			assert.Equal(t, []string{url}, context.Endpoints)
			assert.Equal(t, tt.clusterName, context.Cluster)
			assert.Equal(t, identity, context.Auth.SideroV1.Identity)
		})
	}
}
