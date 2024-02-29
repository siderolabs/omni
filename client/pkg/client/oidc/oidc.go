// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package oidc provides client for Omni OIDC API.
package oidc

import (
	"context"

	"google.golang.org/grpc"

	"github.com/siderolabs/omni/client/api/omni/oidc"
)

// Client for Management API .
type Client struct {
	conn oidc.OIDCServiceClient
}

// NewClient builds a client out of gRPC connection.
func NewClient(conn *grpc.ClientConn) *Client {
	return &Client{
		conn: oidc.NewOIDCServiceClient(conn),
	}
}

// Authenticate confirms the OIDC auth request.
func (client *Client) Authenticate(ctx context.Context, requestID string) (string, error) {
	resp, err := client.conn.Authenticate(ctx,
		&oidc.AuthenticateRequest{
			AuthRequestId: requestID,
		},
	)

	return resp.GetRedirectUrl(), err
}
