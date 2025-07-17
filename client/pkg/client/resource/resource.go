// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package resource provides client for Omni resource API (not COSI API, but the one specific to Omni itself)
package resource

import (
	"context"

	"google.golang.org/grpc"

	"github.com/siderolabs/omni/client/api/omni/resources"
)

// Client for Management API .
type Client struct {
	conn resources.ResourceServiceClient
}

// NewClient builds a client out of gRPC connection.
func NewClient(conn *grpc.ClientConn) *Client {
	return &Client{
		conn: resources.NewResourceServiceClient(conn),
	}
}

// DependencyGraph generates the resource dependency graph.
func (c *Client) DependencyGraph(ctx context.Context, req *resources.DependencyGraphRequest) (*resources.DependencyGraphResponse, error) {
	return c.conn.DependencyGraph(ctx, req)
}
