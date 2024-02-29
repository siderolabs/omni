// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package talos provides helpers for accessing Talos Machine API.
package talos

import (
	"context"

	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/siderolabs/omni/client/api/common"
)

// NewClient wraps gRPC connection interface which adds nodes, cluster name
// to each request.
func NewClient(conn grpc.ClientConnInterface) *Client {
	c := &Client{
		conn: conn,
	}

	c.MachineServiceClient = machine.NewMachineServiceClient(c)

	return c
}

// Client adds runtime, cluster and nodes metadata to all gRPC calls.
type Client struct {
	machine.MachineServiceClient
	conn        grpc.ClientConnInterface
	clusterName string
	nodes       []string
}

// WithCluster adds clusterName to the request metadata.
func (c *Client) WithCluster(clusterName string) *Client {
	c.clusterName = clusterName

	return c
}

// WithNodes adds nodes to the request metadata.
func (c *Client) WithNodes(nodes ...string) *Client {
	c.nodes = nodes

	return c
}

// Invoke performs a unary RPC and returns after the response is received
// into reply.
func (c *Client) Invoke(ctx context.Context, method string, args any, reply any, opts ...grpc.CallOption) error {
	return c.conn.Invoke(c.appendMetadata(ctx), method, args, reply, opts...)
}

// NewStream begins a streaming RPC.
func (c *Client) NewStream(ctx context.Context, desc *grpc.StreamDesc, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	return c.conn.NewStream(c.appendMetadata(ctx), desc, method, opts...)
}

func (c *Client) appendMetadata(ctx context.Context) context.Context {
	ctx = metadata.AppendToOutgoingContext(ctx, "runtime", common.Runtime_Talos.String())

	if c.clusterName != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, "context", c.clusterName)
	}

	if len(c.nodes) > 0 {
		pairs := make([]string, 0, len(c.nodes)*2)
		for _, node := range c.nodes {
			pairs = append(pairs, "nodes", node)
		}

		ctx = metadata.AppendToOutgoingContext(ctx, pairs...)
	}

	return ctx
}
