// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package discovery provides a discovery service client.
package discovery

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/url"
	"time"

	serverpb "github.com/siderolabs/discovery-api/api/v1alpha1/server/pb"
	discoveryclient "github.com/siderolabs/discovery-client/pkg/client"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	callTimeout = 5 * time.Second
	defaultTTL  = 30 * time.Minute
)

// Client is a client for the discovery service.
type Client struct {
	conn          *grpc.ClientConn
	clusterClient serverpb.ClusterClient
}

// NewClient creates a new discovery service client.
func NewClient(endpoint string) (*Client, error) {
	conn, err := createConn(endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection to discovery service: %w", err)
	}

	return &Client{
		conn:          conn,
		clusterClient: serverpb.NewClusterClient(conn),
	}, nil
}

// AffiliateDelete deletes the given affiliate from the given cluster.
func (client *Client) AffiliateDelete(ctx context.Context, cluster, affiliate string) error {
	ctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()

	if _, err := client.clusterClient.AffiliateDelete(ctx, &serverpb.AffiliateDeleteRequest{
		ClusterId:   cluster,
		AffiliateId: affiliate,
	}); err != nil {
		return fmt.Errorf("failed to delete affiliate %q for cluster %q: %w", affiliate, cluster, err)
	}

	return nil
}

// Close closes the underlying connection to the discovery service.
func (client *Client) Close() error {
	return client.conn.Close()
}

// createConn creates a gRPC connection to the discovery service.
func createConn(endpoint string) (*grpc.ClientConn, error) {
	var (
		transportCredentials credentials.TransportCredentials
		host, port           string
	)

	parsed, err := url.ParseRequestURI(endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to parse discovery service URL %q: %w", endpoint, err)
	}

	switch parsed.Scheme {
	case "http":
		transportCredentials = insecure.NewCredentials()
		host = parsed.Hostname()

		port = parsed.Port()
		if port == "" {
			port = "80"
		}
	case "https":
		transportCredentials = credentials.NewTLS(&tls.Config{})
		host = parsed.Hostname()

		port = parsed.Port()
		if port == "" {
			port = "443"
		}
	default:
		return nil, fmt.Errorf("unsupported scheme %q for discovery service URL %q", parsed.Scheme, endpoint)
	}

	target := net.JoinHostPort(host, port)
	opts := discoveryclient.GRPCDialOptions(discoveryclient.Options{
		TTL: defaultTTL,
	})

	opts = append(opts, grpc.WithSharedWriteBuffer(true), grpc.WithTransportCredentials(transportCredentials))

	discoveryConn, err := grpc.NewClient(target, opts...)
	if err != nil {
		return nil, err
	}

	return discoveryConn, nil
}
