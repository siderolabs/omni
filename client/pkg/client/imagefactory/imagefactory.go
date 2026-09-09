// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package imagefactory provides a client for Omni's ImageFactoryService: the security artifacts
// (vulnerability scans, SBOMs, VEX documents) Omni proxies from the image factory on the caller's
// behalf.
package imagefactory

import (
	"context"

	"google.golang.org/grpc"

	"github.com/siderolabs/omni/client/api/omni/imagefactory"
)

// Client for the ImageFactoryService API.
type Client struct {
	conn imagefactory.ImageFactoryServiceClient
}

// NewClient builds a client out of a gRPC connection.
func NewClient(conn *grpc.ClientConn) *Client {
	return &Client{
		conn: imagefactory.NewImageFactoryServiceClient(conn),
	}
}

// VulnerabilityReport returns the vulnerability scan report of a schematic.
func (client *Client) VulnerabilityReport(
	ctx context.Context, req *imagefactory.VulnerabilityReportRequest,
) (*imagefactory.VulnerabilityReportResponse, error) {
	return client.conn.VulnerabilityReport(ctx, req)
}

// SBOM returns the SPDX bundle of a schematic.
func (client *Client) SBOM(ctx context.Context, req *imagefactory.SBOMRequest) (*imagefactory.SBOMResponse, error) {
	return client.conn.SBOM(ctx, req)
}

// VEXDocument returns the VEX document of a Talos version.
func (client *Client) VEXDocument(
	ctx context.Context, req *imagefactory.VEXDocumentRequest,
) (*imagefactory.VEXDocumentResponse, error) {
	return client.conn.VEXDocument(ctx, req)
}

// ClusterArtifactTargets resolves the (schematic, arch) pairs installed across a cluster's
// machines, along with the Talos versions to fetch security artifacts for.
func (client *Client) ClusterArtifactTargets(
	ctx context.Context, req *imagefactory.ClusterArtifactTargetsRequest,
) (*imagefactory.ClusterArtifactTargetsResponse, error) {
	return client.conn.ClusterArtifactTargets(ctx, req)
}
