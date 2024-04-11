// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package discovery

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/url"
	"slices"
	"time"

	serverpb "github.com/siderolabs/discovery-api/api/v1alpha1/server/pb"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

const callTimeout = 10 * time.Second

// Client is a client for the discovery service.
type Client struct {
	conn          *grpc.ClientConn
	clusterClient serverpb.ClusterClient
}

// NewClient creates a new discovery service client.
func NewClient(ctx context.Context) (*Client, error) {
	conn, err := createConn(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection to discovery service: %w", err)
	}

	return &Client{
		conn:          conn,
		clusterClient: serverpb.NewClusterClient(conn),
	}, nil
}

// ListAffiliates returns the list of affiliates for the given cluster.
func (client *Client) ListAffiliates(ctx context.Context, cluster string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()

	listResponse, err := client.clusterClient.List(ctx, &serverpb.ListRequest{ClusterId: cluster})
	if err != nil {
		return nil, fmt.Errorf("failed to list discovery affiliates for cluster %q: %w", cluster, err)
	}

	ids := xslices.Map(listResponse.GetAffiliates(), func(affiliate *serverpb.Affiliate) string {
		return affiliate.GetId()
	})

	slices.Sort(ids)

	return ids, nil
}

// DeleteAffiliate deletes the given affiliate from the given cluster.
func (client *Client) DeleteAffiliate(ctx context.Context, cluster, affiliate string) error {
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
func createConn(ctx context.Context) (*grpc.ClientConn, error) {
	u, err := url.Parse(constants.DefaultDiscoveryServiceEndpoint)
	if err != nil {
		return nil, err
	}

	discoveryConn, err := grpc.DialContext(ctx, net.JoinHostPort(u.Host, "443"),
		grpc.WithTransportCredentials(
			credentials.NewTLS(&tls.Config{}),
		),
		grpc.WithSharedWriteBuffer(true),
	)
	if err != nil {
		return nil, err
	}

	return discoveryConn, nil
}
