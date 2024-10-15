// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package omni provides client for Omni resource access.
package omni

import (
	"context"

	"github.com/cosi-project/runtime/api/v1alpha1"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/protobuf/client"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/siderolabs/omni/client/pkg/constants"
	_ "github.com/siderolabs/omni/client/pkg/omni/resources/auth" // import resources to register protobufs
	_ "github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	_ "github.com/siderolabs/omni/client/pkg/omni/resources/k8s"
	_ "github.com/siderolabs/omni/client/pkg/omni/resources/oidc"
	_ "github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	_ "github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	_ "github.com/siderolabs/omni/client/pkg/omni/resources/system"
	_ "github.com/siderolabs/omni/client/pkg/omni/resources/virtual"
)

// Options defines additional Omni client options.
type Options struct {
	infraProviderID string
}

// Option define an additional Omni client option.
type Option func(*Options)

// WithProviderID sets provider ID to the metadata of each request.
func WithProviderID(id string) Option {
	return func(o *Options) {
		o.infraProviderID = id
	}
}

// Client for Omni resource API (COSI).
type Client struct {
	conn    *grpc.ClientConn
	state   state.State
	options Options
}

// NewClient builds a client out of gRPC connection.
func NewClient(conn *grpc.ClientConn, options ...Option) *Client {
	c := &Client{
		conn: conn,
	}

	for _, o := range options {
		o(&c.options)
	}

	c.state = state.WrapCore(client.NewAdapter(v1alpha1.NewStateClient(c)))

	return c
}

// State provides access to the COSI resource state.
func (client *Client) State() state.State { //nolint:ireturn
	return client.state
}

// Invoke performs a unary RPC and returns after the response is received
// into reply.
func (client *Client) Invoke(ctx context.Context, method string, args any, reply any, opts ...grpc.CallOption) error {
	return client.conn.Invoke(client.appendMetadata(ctx), method, args, reply, opts...)
}

// NewStream begins a streaming RPC.
func (client *Client) NewStream(ctx context.Context, desc *grpc.StreamDesc, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	return client.conn.NewStream(client.appendMetadata(ctx), desc, method, opts...)
}

func (client *Client) appendMetadata(ctx context.Context) context.Context {
	if client.options.infraProviderID != "" {
		return metadata.AppendToOutgoingContext(ctx, constants.InfraProviderMetadataKey, client.options.infraProviderID)
	}

	return ctx
}
