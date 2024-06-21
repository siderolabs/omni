// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package omni provides client for Omni resource access.
package omni

import (
	"github.com/cosi-project/runtime/api/v1alpha1"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/protobuf/client"
	"google.golang.org/grpc"

	_ "github.com/siderolabs/omni/client/pkg/omni/resources/auth" // import resources to register protobufs
	_ "github.com/siderolabs/omni/client/pkg/omni/resources/cloud"
	_ "github.com/siderolabs/omni/client/pkg/omni/resources/k8s"
	_ "github.com/siderolabs/omni/client/pkg/omni/resources/oidc"
	_ "github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	_ "github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	_ "github.com/siderolabs/omni/client/pkg/omni/resources/system"
	_ "github.com/siderolabs/omni/client/pkg/omni/resources/virtual"
)

// Client for Omni resource API (COSI).
type Client struct {
	state state.State
}

// NewClient builds a client out of gRPC connection.
func NewClient(conn *grpc.ClientConn) *Client {
	return &Client{
		state: state.WrapCore(client.NewAdapter(v1alpha1.NewStateClient(conn))),
	}
}

// State provides access to the COSI resource state.
func (client *Client) State() state.State { //nolint:ireturn
	return client.state
}
