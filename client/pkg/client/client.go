// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package client provides Omni API client.
package client

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/url"
	"slices"

	"github.com/siderolabs/go-api-signature/pkg/client/auth"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/encoding/gzip"

	"github.com/siderolabs/omni/client/pkg/client/management"
	"github.com/siderolabs/omni/client/pkg/client/oidc"
	"github.com/siderolabs/omni/client/pkg/client/omni"
	"github.com/siderolabs/omni/client/pkg/client/talos"
	"github.com/siderolabs/omni/client/pkg/compression"
	"github.com/siderolabs/omni/client/pkg/constants"
)

// Client is Omni API client.
type Client struct {
	conn *grpc.ClientConn

	endpoint string
}

// New creates a new Omni API client.
func New(endpoint string, opts ...Option) (*Client, error) {
	if err := compression.InitConfig(true); err != nil {
		return nil, fmt.Errorf("failed to initialize compression config: %w", err)
	}

	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}

	if u.Port() == "" && u.Scheme == "https" {
		u.Host = net.JoinHostPort(u.Host, "443")
	}

	if u.Scheme == "http" {
		u.Scheme = "grpc"
	}

	if u.Port() == "" && u.Scheme == "grpc" {
		u.Host = net.JoinHostPort(u.Host, "80")
	}

	var (
		options         Options
		grpcDialOptions []grpc.DialOption
	)

	for _, opt := range opts {
		opt(&options)
	}

	if options.AuthInterceptor != nil {
		grpcDialOptions = append(grpcDialOptions,
			grpc.WithUnaryInterceptor(options.AuthInterceptor.Unary()),
			grpc.WithStreamInterceptor(options.AuthInterceptor.Stream()))
	}

	grpcDialOptions = slices.Concat(grpcDialOptions, options.AdditionalGRPCDialOptions)

	switch u.Scheme {
	case "https":
		grpcDialOptions = append(grpcDialOptions, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{
			InsecureSkipVerify: options.InsecureSkipTLSVerify,
		})))
	default:
		grpcDialOptions = append(grpcDialOptions, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	grpcDialOptions = append(grpcDialOptions,
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(constants.GRPCMaxMessageSize),
			grpc.UseCompressor(gzip.Name),
		),
		grpc.WithSharedWriteBuffer(true),
	)

	c := &Client{
		endpoint: u.String(),
	}

	c.conn, err = grpc.NewClient(u.Host, grpcDialOptions...)
	if err != nil {
		return nil, err
	}

	return c, nil
}

// Close the client.
func (c *Client) Close() error {
	return c.conn.Close()
}

// Omni provides access to Omni resource API.
func (c *Client) Omni() *omni.Client {
	return omni.NewClient(c.conn)
}

// Management provides access to the management API.
func (c *Client) Management() *management.Client {
	return management.NewClient(c.conn)
}

// OIDC provides access to the OIDC API.
func (c *Client) OIDC() *oidc.Client {
	return oidc.NewClient(c.conn)
}

// Auth provides access to the auth API.
func (c *Client) Auth() *auth.Client {
	return auth.NewClient(c.conn)
}

// Talos provides access to Talos machine API.
func (c *Client) Talos() *talos.Client {
	return talos.NewClient(c.conn)
}

// Endpoint returns the endpoint this client is configured to talk to.
func (c *Client) Endpoint() string {
	return c.endpoint
}
