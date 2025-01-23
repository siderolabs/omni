// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
)

type listImagesRequest struct {
	cluster string
	node    string
}

type pullImageRequest struct {
	cluster string
	node    string
	image   string
}

type mockImageClient struct {
	state state.State

	listRequests []listImagesRequest
	pullRequests []pullImageRequest

	lock sync.Mutex
}

func (m *mockImageClient) ListImagesOnNode(_ context.Context, cluster, node string) ([]string, error) {
	m.lock.Lock()
	defer m.lock.Unlock()

	m.listRequests = append(m.listRequests, listImagesRequest{
		cluster: cluster,
		node:    node,
	})

	return []string{node + "-image-1", node + "-image-4"}, nil // mimic <node>-image-1 and image-4 being already on the node
}

func (m *mockImageClient) PullImageToNode(_ context.Context, cluster, node, image string) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	m.pullRequests = append(m.pullRequests, pullImageRequest{
		cluster: cluster,
		node:    node,
		image:   image,
	})

	return nil
}

func TestImagePullStatusControllerSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(ImagePullStatusControllerSuite))
}

type ImagePullStatusControllerSuite struct {
	OmniSuite

	imageClient *mockImageClient
}

func (suite *ImagePullStatusControllerSuite) SetupTest() {
	suite.OmniSuite.SetupTest()

	suite.startRuntime()

	suite.imageClient = &mockImageClient{
		state: suite.state,
	}

	imagePullStatusController := omnictrl.NewImagePullStatusController(suite.imageClient)

	suite.Require().NoError(suite.runtime.RegisterController(imagePullStatusController))
}

func (suite *ImagePullStatusControllerSuite) TestImagePullStatus() {
	pr1 := omni.NewImagePullRequest(resources.DefaultNamespace, "pr-1")

	pr1.Metadata().Labels().Set(omni.LabelCluster, "pr-1-cluster")

	pr1.TypedSpec().Value.NodeImageList = []*specs.ImagePullRequestSpec_NodeImageList{
		{
			Node:   "node-1",
			Images: []string{"node-1-image-1", "node-1-image-2"},
		},
		{
			Node:   "node-2",
			Images: []string{"node-2-image-1", "node-2-image-2", "node-2-image-3", "node-2-image-4"},
		},
	}

	pr2 := omni.NewImagePullRequest(resources.DefaultNamespace, "pr-2")

	pr2.Metadata().Labels().Set(omni.LabelCluster, "pr-2-cluster")

	pr2.TypedSpec().Value.NodeImageList = []*specs.ImagePullRequestSpec_NodeImageList{
		{
			Node:   "node-3",
			Images: []string{"node-3-image-1", "node-3-image-2"},
		},
	}

	// create two pull requests
	suite.Require().NoError(suite.state.Create(suite.ctx, pr1))
	suite.Require().NoError(suite.state.Create(suite.ctx, pr2))

	suite.Require().EventuallyWithT(func(collect *assert.CollectT) {
		suite.imageClient.lock.Lock()
		defer suite.imageClient.lock.Unlock()

		// assert that images were listed
		if assert.Len(collect, suite.imageClient.listRequests, 3) {
			assert.ElementsMatch(collect, []listImagesRequest{
				{cluster: "pr-1-cluster", node: "node-1"},
				{cluster: "pr-1-cluster", node: "node-2"},
				{cluster: "pr-2-cluster", node: "node-3"},
			}, suite.imageClient.listRequests)
		}

		// assert that only the missing images were pulled
		if assert.Len(collect, suite.imageClient.pullRequests, 4) {
			assert.ElementsMatch(collect, []pullImageRequest{
				// node-1-image-1 is already on the node, so it should not be pulled
				{cluster: "pr-1-cluster", node: "node-1", image: "node-1-image-2"},
				// node-2-image-1 is already on the node, so it should not be pulled
				{cluster: "pr-1-cluster", node: "node-2", image: "node-2-image-2"},
				{cluster: "pr-1-cluster", node: "node-2", image: "node-2-image-3"},
				// node-3-image-1 is already on the node, so it should not be pulled
				{cluster: "pr-2-cluster", node: "node-3", image: "node-3-image-2"},
			}, suite.imageClient.pullRequests)
		}

		// assert the ImagePullStatus at the end

		sts1, err := safe.StateGet[*omni.ImagePullStatus](suite.ctx, suite.state, omni.NewImagePullStatus(resources.DefaultNamespace, pr1.Metadata().ID()).Metadata())
		assert.NoError(collect, err)

		if sts1 == nil { // not there yet
			return
		}

		sts1Cluster, _ := sts1.Metadata().Labels().Get(omni.LabelCluster)
		assert.Equal(collect, "pr-1-cluster", sts1Cluster)

		// pr-1 will pull four images, so the version should be 4
		assert.Equal(collect, "4", sts1.Metadata().Version().String())

		assert.Equal(collect, sts1.TypedSpec().Value.GetRequestVersion(), pr1.Metadata().Version().String())
		assert.Equal(collect, sts1.TypedSpec().Value.GetLastProcessedNode(), "node-2")
		assert.Equal(collect, sts1.TypedSpec().Value.GetLastProcessedImage(), "node-2-image-4")
		assert.Equal(collect, sts1.TypedSpec().Value.GetProcessedCount(), uint32(6)) // the processed count also includes images already on the node
		assert.Equal(collect, sts1.TypedSpec().Value.GetTotalCount(), uint32(6))     // the total count also includes images already on the node
		assert.Equal(collect, sts1.TypedSpec().Value.GetLastProcessedError(), "")

		sts2, err := safe.StateGet[*omni.ImagePullStatus](suite.ctx, suite.state, omni.NewImagePullStatus(resources.DefaultNamespace, pr2.Metadata().ID()).Metadata())
		assert.NoError(collect, err)

		if sts2 == nil { // not there yet
			return
		}

		sts2Cluster, _ := sts2.Metadata().Labels().Get(omni.LabelCluster)
		assert.Equal(collect, "pr-2-cluster", sts2Cluster)

		// pr-2 will pull a single image, so the version should be 1
		assert.Equal(collect, "1", sts2.Metadata().Version().String())

		assert.Equal(collect, sts2.TypedSpec().Value.GetRequestVersion(), pr2.Metadata().Version().String())
		assert.Equal(collect, sts2.TypedSpec().Value.GetLastProcessedNode(), "node-3")
		assert.Equal(collect, sts2.TypedSpec().Value.GetLastProcessedImage(), "node-3-image-2")
		assert.Equal(collect, sts2.TypedSpec().Value.GetProcessedCount(), uint32(2)) // the processed count also includes images already on the node
		assert.Equal(collect, sts2.TypedSpec().Value.GetTotalCount(), uint32(2))     // the total count also includes images already on the node
		assert.Equal(collect, sts2.TypedSpec().Value.GetLastProcessedError(), "")
	}, 10*time.Second, 500*time.Microsecond)

	// update pr-2 new a new image list and expect it to be processed

	pr2.TypedSpec().Value.NodeImageList = []*specs.ImagePullRequestSpec_NodeImageList{
		{
			Node:   "node-4",
			Images: []string{"node-4-image-1", "node-4-image-2"},
		},
	}

	suite.Require().NoError(suite.state.Update(suite.ctx, pr2))

	suite.Require().EventuallyWithT(func(collect *assert.CollectT) {
		suite.imageClient.lock.Lock()
		defer suite.imageClient.lock.Unlock()

		if assert.Len(collect, suite.imageClient.listRequests, 4) {
			assert.Equal(collect, listImagesRequest{cluster: "pr-2-cluster", node: "node-4"}, suite.imageClient.listRequests[3])
		}

		if assert.Len(collect, suite.imageClient.pullRequests, 5) {
			assert.Equal(collect, pullImageRequest{cluster: "pr-2-cluster", node: "node-4", image: "node-4-image-2"}, suite.imageClient.pullRequests[4])
		}

		sts2, err := safe.StateGet[*omni.ImagePullStatus](suite.ctx, suite.state, omni.NewImagePullStatus(resources.DefaultNamespace, pr2.Metadata().ID()).Metadata())
		assert.NoError(collect, err)

		assert.Equal(collect, sts2.TypedSpec().Value.GetRequestVersion(), pr2.Metadata().Version().String())
		assert.Equal(collect, sts2.TypedSpec().Value.GetLastProcessedNode(), "node-4")
		assert.Equal(collect, sts2.TypedSpec().Value.GetLastProcessedImage(), "node-4-image-2")
		assert.Equal(collect, sts2.TypedSpec().Value.GetProcessedCount(), uint32(2)) // the processed count also includes images already on the node
		assert.Equal(collect, sts2.TypedSpec().Value.GetTotalCount(), uint32(2))     // the total count also includes images already on the node
		assert.Equal(collect, sts2.TypedSpec().Value.GetLastProcessedError(), "")
	}, 10*time.Second, 500*time.Microsecond)
}
