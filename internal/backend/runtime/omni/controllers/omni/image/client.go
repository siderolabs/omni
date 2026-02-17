// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package image

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/siderolabs/talos/pkg/machinery/api/common"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"

	"github.com/siderolabs/omni/internal/backend/grpc/router"
	"github.com/siderolabs/omni/internal/backend/runtime/talos"
)

// Client is the interface for interacting with the container images on a Talos machine.
type Client interface {
	ListImagesOnNode(ctx context.Context, cluster, node string) ([]string, error)
	PullImageToNode(ctx context.Context, cluster, node, image string) error
}

// TalosImageClient implements kubernetes.ImageClient interface.
type TalosImageClient struct {
	TalosClientFactory *talos.ClientFactory
	NodeResolver       router.NodeResolver
}

// ListImagesOnNode lists images on a node.
func (c *TalosImageClient) ListImagesOnNode(ctx context.Context, cluster, node string) ([]string, error) {
	talosCli, err := c.TalosClientFactory.GetForCluster(ctx, cluster)
	if err != nil {
		return nil, fmt.Errorf("failed to get talos client: %w", err)
	}

	nodeAddress := c.NodeResolver.Resolve(cluster, node).GetAddress()

	//nolint:staticcheck
	imageListStream, err := talosCli.ImageList(client.WithNode(ctx, nodeAddress), common.ContainerdNamespace_NS_CRI)
	if err != nil {
		return nil, fmt.Errorf("failed to list images: %w", err)
	}

	return c.readImagesFromStream(imageListStream)
}

func (c *TalosImageClient) readImagesFromStream(stream machine.MachineService_ImageListClient) ([]string, error) {
	var images []string

	for {
		item, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return images, nil
			}

			return nil, fmt.Errorf("error streaming results: %w", err)
		}

		images = append(images, item.GetName())
	}
}

// PullImageToNode pulls the given image to the given node.
func (c *TalosImageClient) PullImageToNode(ctx context.Context, cluster, node, image string) error {
	talosCli, err := c.TalosClientFactory.GetForCluster(ctx, cluster)
	if err != nil {
		return fmt.Errorf("failed to get talos client: %w", err)
	}

	nodeAddress := c.NodeResolver.Resolve(cluster, node).GetAddress()
	if nodeAddress == "" {
		return fmt.Errorf("failed to resolve node %q", node)
	}

	//nolint:staticcheck
	if err = talosCli.ImagePull(client.WithNode(ctx, nodeAddress), common.ContainerdNamespace_NS_CRI, image); err != nil {
		return fmt.Errorf("failed to pull image %s: %w", image, err)
	}

	return nil
}
