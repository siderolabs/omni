// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
)

type DiscoveryAffiliateDeleteTaskSuite struct {
	OmniSuite
}

func (suite *DiscoveryAffiliateDeleteTaskSuite) TestReconcile() {
	suite.startRuntime()

	deleteCh := make(chan affiliateDelete)
	clock := clockwork.NewFakeClock()
	discoveryClientCache := &discoveryClientCacheMock{
		deleteCh: deleteCh,
	}
	controller := omnictrl.NewDiscoveryAffiliateDeleteTaskController(clock, discoveryClientCache)

	suite.Require().NoError(suite.runtime.RegisterQController(controller))

	task := omni.NewDiscoveryAffiliateDeleteTask("affiliate1")
	task.TypedSpec().Value.ClusterId = "cluster1"
	task.TypedSpec().Value.DiscoveryServiceEndpoint = "endpoint1"

	suite.Require().NoError(suite.state.Create(suite.ctx, task, state.WithCreateOwner(omnictrl.ClusterMachineTeardownControllerName)))

	// capture the single deletion attempt and assert the values
	select {
	case <-suite.ctx.Done():
		suite.Require().Fail("reconciliation timed out")
	case affDelete := <-deleteCh:
		suite.Equal("endpoint1", affDelete.endpoint)
		suite.Equal("cluster1", affDelete.cluster)
		suite.Equal("affiliate1", affDelete.affiliate)
	}

	// assert that the delete task is cleaned up
	rtestutils.AssertNoResource[*omni.DiscoveryAffiliateDeleteTask](suite.ctx, suite.T(), suite.state, task.Metadata().ID())
}

func (suite *DiscoveryAffiliateDeleteTaskSuite) TestExpiration() {
	suite.startRuntime()

	deleteCh := make(chan affiliateDelete, 128) // use a large enough buffer to not block retries of the controller
	clock := clockwork.NewFakeClock()
	discoveryClientCache := &discoveryClientCacheMock{
		deleteCh:     deleteCh,
		failDeletion: true, // block deletion - simulate discovery service being not accessible
	}
	controller := omnictrl.NewDiscoveryAffiliateDeleteTaskController(clock, discoveryClientCache)

	suite.Require().NoError(suite.runtime.RegisterQController(controller))

	task := omni.NewDiscoveryAffiliateDeleteTask("affiliate1")
	task.TypedSpec().Value.ClusterId = "cluster1"
	task.TypedSpec().Value.DiscoveryServiceEndpoint = "endpoint1"

	suite.Require().NoError(suite.state.Create(suite.ctx, task, state.WithCreateOwner(omnictrl.ClusterMachineTeardownControllerName)))

	// advance time, but not enough for the affiliate to be assumed to be pruned by the discovery service itself
	clock.Advance(10 * time.Minute)

	// capture a deletion attempt and assert the values
	select {
	case <-suite.ctx.Done():
		suite.Require().Fail("timed out")
	case affDelete := <-deleteCh:
		suite.Equal("endpoint1", affDelete.endpoint)
		suite.Equal("cluster1", affDelete.cluster)
		suite.Equal("affiliate1", affDelete.affiliate)
	}

	// capture another deletion attempt, since the first one has failed, and re-assert the values
	select {
	case <-suite.ctx.Done():
		suite.Require().Fail("timed out")
	case affDelete := <-deleteCh:
		suite.Equal("endpoint1", affDelete.endpoint)
		suite.Equal("cluster1", affDelete.cluster)
		suite.Equal("affiliate1", affDelete.affiliate)
	}

	// assert that the task is still there, as the deletion failed
	rtestutils.AssertResource[*omni.DiscoveryAffiliateDeleteTask](suite.ctx, suite.T(), suite.state, task.Metadata().ID(),
		func(r *omni.DiscoveryAffiliateDeleteTask, assertion *assert.Assertions) {
			assertion.Equal(resource.PhaseRunning, r.Metadata().Phase())
		},
	)

	// advance the time further so that the affiliate is assumed to be pruned by the discovery service itself and a delete call will not be attempted
	clock.Advance(21 * time.Minute)

	// assert that the delete task is cleaned up
	rtestutils.AssertNoResource[*omni.DiscoveryAffiliateDeleteTask](suite.ctx, suite.T(), suite.state, task.Metadata().ID())
}

func TestDiscoveryAffiliateDeleteTaskSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(DiscoveryAffiliateDeleteTaskSuite))
}

type affiliateDelete struct {
	endpoint  string
	cluster   string
	affiliate string
}

type discoveryClientCacheMock struct {
	deleteCh     chan affiliateDelete
	failDeletion bool
}

func (d *discoveryClientCacheMock) AffiliateDelete(ctx context.Context, endpoint, cluster, affiliate string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case d.deleteCh <- affiliateDelete{endpoint, cluster, affiliate}:
	}

	if d.failDeletion {
		return fmt.Errorf("deletion blocked - discovery service is not accessible")
	}

	return nil
}
