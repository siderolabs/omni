// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package image

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/task"
	"github.com/hashicorp/go-multierror"
	"github.com/siderolabs/gen/channel"
	"github.com/siderolabs/gen/xslices"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/image"
)

const (
	listImagesTimeout = 15 * time.Second
	pullImageTimeout  = 5 * time.Minute
)

// PullTaskSpec represents the spec of an image pull task.
type PullTaskSpec struct {
	request *omni.ImagePullRequest

	imageClient image.Client
}

// NewPullTaskSpec creates new PullTaskSpec.
func NewPullTaskSpec(request *omni.ImagePullRequest, imageClient image.Client) PullTaskSpec {
	return PullTaskSpec{
		request:     request,
		imageClient: imageClient,
	}
}

// ID returns the ID of the pull task.
func (p PullTaskSpec) ID() task.ID {
	return p.request.Metadata().ID()
}

// RunTask runs the pull task.
func (p PullTaskSpec) RunTask(ctx context.Context, _ *zap.Logger, pullStatusCh PullStatusChan) error {
	var (
		currentNum, totalNum      int
		currentNode, currentImage string
		currentError              error
	)

	clusterID, ok := p.request.Metadata().Labels().Get(omni.LabelCluster)
	if !ok {
		return fmt.Errorf("missing cluster label on %q", p.request.Metadata())
	}

	nodeImageInfoMap, err := p.getNodeImageInfos(ctx, p.request, clusterID)
	if err != nil {
		return err
	}

	for _, imageList := range p.request.TypedSpec().Value.GetNodeImageList() {
		totalNum += len(imageList.Images)
	}

	var errs error

	for _, imageList := range p.request.TypedSpec().Value.GetNodeImageList() {
		currentNode = imageList.Node

		for _, img := range imageList.Images {
			currentError = nil
			currentImage = img

			if !nodeImageInfoMap.shouldPull(imageList.Node, img) {
				currentNum++

				continue
			}

			currentError = p.pullImage(ctx, clusterID, imageList.Node, img)
			if currentError == nil {
				currentNum++
			} else {
				errs = multierror.Append(errs, currentError)
			}

			if !channel.SendWithContext(ctx, pullStatusCh, PullStatus{
				Request:    p.request,
				Node:       currentNode,
				Image:      currentImage,
				CurrentNum: currentNum,
				TotalNum:   totalNum,
				Error:      currentError,
			}) {
				return errs
			}
		}
	}

	channel.SendWithContext(ctx, pullStatusCh, PullStatus{
		Request:    p.request,
		Node:       currentNode,
		Image:      currentImage,
		CurrentNum: currentNum,
		TotalNum:   totalNum,
		Error:      currentError,
	})

	return errs
}

// Equal returns true if the pull task spec is equal to the other pull task spec.
func (p PullTaskSpec) Equal(other PullTaskSpec) bool {
	if p.request.Metadata().ID() != other.request.Metadata().ID() {
		return false
	}

	nodeImageList := p.request.TypedSpec().Value.GetNodeImageList()
	otherNodeImageList := other.request.TypedSpec().Value.GetNodeImageList()

	if len(nodeImageList) != len(otherNodeImageList) {
		return false
	}

	for i, imageList := range nodeImageList {
		otherImageList := otherNodeImageList[i]

		if imageList.Node != otherImageList.Node {
			return false
		}

		if !slices.Equal(imageList.Images, otherImageList.Images) {
			return false
		}
	}

	return true
}

func (p PullTaskSpec) pullImage(ctx context.Context, clusterID resource.ID, node, image string) error {
	ctx, cancel := context.WithTimeout(ctx, pullImageTimeout)
	defer cancel()

	if err := p.imageClient.PullImageToNode(ctx, clusterID, node, image); err != nil {
		return fmt.Errorf("failed to pull image %q to node %q", image, node)
	}

	return nil
}

func (p PullTaskSpec) getNodeImageInfos(ctx context.Context, request *omni.ImagePullRequest, clusterID string) (nodeImageInfos, error) {
	nodeToNodeImageInfo := make(map[string]nodeImageInfo)

	for _, images := range request.TypedSpec().Value.GetNodeImageList() {
		if _, exists := nodeToNodeImageInfo[images.Node]; exists {
			continue
		}

		imageSet, err := p.getImageSetOnNode(ctx, clusterID, images.Node)
		if err != nil {
			return nodeImageInfos{}, err
		}

		nodeToNodeImageInfo[images.Node] = imageSet
	}

	return nodeImageInfos{nodeToNodeImageInfo: nodeToNodeImageInfo}, nil
}

func (p PullTaskSpec) getImageSetOnNode(ctx context.Context, clusterID resource.ID, node string) (nodeImageInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, listImagesTimeout)
	defer cancel()

	images, err := p.imageClient.ListImagesOnNode(ctx, clusterID, node)
	if err != nil {
		if status.Code(err) == codes.Unimplemented {
			return nodeImageInfo{
				supportsImagePull: false,
			}, nil
		}

		return nodeImageInfo{}, fmt.Errorf("failed to list images on node %q: %w", node, err)
	}

	return nodeImageInfo{
		existingImageSet:  xslices.ToSet(images),
		supportsImagePull: true,
	}, nil
}

type nodeImageInfo struct {
	existingImageSet  map[string]struct{}
	supportsImagePull bool
}

type nodeImageInfos struct {
	nodeToNodeImageInfo map[string]nodeImageInfo
}

func (i *nodeImageInfos) shouldPull(node, image string) bool {
	imageSet, ok := i.nodeToNodeImageInfo[node]
	if !ok {
		return false
	}

	if !imageSet.supportsImagePull {
		return false
	}

	_, exists := imageSet.existingImageSet[image]

	return !exists
}
