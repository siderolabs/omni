// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/siderolabs/gen/maps"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
)

type DiscoveryAuditSuite struct {
	OmniSuite
}

func (suite *DiscoveryAuditSuite) TestReconcile() {
	suite.startRuntime()

	discoveryClient := &discoveryClientMock{
		affiliates: map[string]map[string]struct{}{
			"cluster1": {"affiliate1": {}, "affiliate2": {}, "affiliate3": {}, "affiliate4": {}, "affiliate5": {}},
			"cluster2": {"affiliate1": {}, "affiliate2": {}, "affiliate3": {}},
		},
	}

	createDiscoveryClient := func(context.Context) (omnictrl.DiscoveryClient, error) {
		return discoveryClient, nil
	}

	discoveryAuditController := omnictrl.NewDiscoveryAuditController(createDiscoveryClient, 2*time.Second)

	suite.Require().NoError(suite.runtime.RegisterQController(discoveryAuditController))

	// create cluster1 that has an affiliate mismatch with the discovery service

	clusterIdentity1 := omni.NewClusterIdentity(resources.DefaultNamespace, "cluster1-test")

	clusterIdentity1.TypedSpec().Value.ClusterId = "cluster1"
	clusterIdentity1.TypedSpec().Value.NodeIds = []string{"affiliate1", "affiliate2", "affiliate3"}

	// create cluster2 that has no affiliate mismatch with the discovery service

	clusterIdentity2 := omni.NewClusterIdentity(resources.DefaultNamespace, "cluster2-test")

	clusterIdentity2.TypedSpec().Value.ClusterId = "cluster2"
	clusterIdentity2.TypedSpec().Value.NodeIds = []string{"affiliate1", "affiliate2", "affiliate3"}

	suite.Require().NoError(suite.state.Create(suite.ctx, clusterIdentity1))
	suite.Require().NoError(suite.state.Create(suite.ctx, clusterIdentity2))

	// assert the audit results

	assertResource[*omni.DiscoveryAuditResult](&suite.OmniSuite, omni.NewDiscoveryAuditResult(resources.DefaultNamespace, "cluster1-test").Metadata(),
		func(res *omni.DiscoveryAuditResult, assertion *assert.Assertions) {
			assertion.Equal("cluster1", res.TypedSpec().Value.ClusterId)
			assertion.Equal([]string{"affiliate4", "affiliate5"}, res.TypedSpec().Value.DeletedAffiliateIds)
		})

	assertNoResource[*omni.DiscoveryAuditResult](&suite.OmniSuite, omni.NewDiscoveryAuditResult(resources.DefaultNamespace, "cluster2-test"))

	// assert the discovery service state

	cluster1Affiliates, err := discoveryClient.ListAffiliates(suite.ctx, "cluster1")
	suite.Require().NoError(err)

	suite.Equal([]string{"affiliate1", "affiliate2", "affiliate3"}, cluster1Affiliates)

	cluster2Affiliates, err := discoveryClient.ListAffiliates(suite.ctx, "cluster2")
	suite.Require().NoError(err)

	suite.Equal([]string{"affiliate1", "affiliate2", "affiliate3"}, cluster2Affiliates)

	// update both clusters to create a mismatch with the discovery service, namely, make "affiliate3" invalid for both of them

	_, err = safe.StateUpdateWithConflicts[*omni.ClusterIdentity](suite.ctx, suite.state, clusterIdentity1.Metadata(), func(res *omni.ClusterIdentity) error {
		res.TypedSpec().Value.NodeIds = []string{"affiliate1", "affiliate2"}

		return nil
	})
	suite.Require().NoError(err)

	_, err = safe.StateUpdateWithConflicts[*omni.ClusterIdentity](suite.ctx, suite.state, clusterIdentity2.Metadata(), func(res *omni.ClusterIdentity) error {
		res.TypedSpec().Value.NodeIds = []string{"affiliate1", "affiliate2"}

		return nil
	})
	suite.Require().NoError(err)

	// assert the audit results

	assertResource[*omni.DiscoveryAuditResult](&suite.OmniSuite, omni.NewDiscoveryAuditResult(resources.DefaultNamespace, "cluster1-test").Metadata(),
		func(res *omni.DiscoveryAuditResult, assertion *assert.Assertions) {
			assertion.Equal("cluster1", res.TypedSpec().Value.ClusterId)
			assertion.Equal([]string{"affiliate3"}, res.TypedSpec().Value.DeletedAffiliateIds)
		})

	assertResource[*omni.DiscoveryAuditResult](&suite.OmniSuite, omni.NewDiscoveryAuditResult(resources.DefaultNamespace, "cluster2-test").Metadata(),
		func(res *omni.DiscoveryAuditResult, assertion *assert.Assertions) {
			assertion.Equal("cluster2", res.TypedSpec().Value.ClusterId)
			assertion.Equal([]string{"affiliate3"}, res.TypedSpec().Value.DeletedAffiliateIds)
		})

	// assert the discovery service state

	cluster1Affiliates, err = discoveryClient.ListAffiliates(suite.ctx, "cluster1")
	suite.Require().NoError(err)

	suite.Equal([]string{"affiliate1", "affiliate2"}, cluster1Affiliates)

	cluster2Affiliates, err = discoveryClient.ListAffiliates(suite.ctx, "cluster2")
	suite.Require().NoError(err)

	suite.Equal([]string{"affiliate1", "affiliate2"}, cluster2Affiliates)

	// destroy cluster1 and assert that all affiliates are deleted from the discovery service

	rtestutils.Destroy[*omni.ClusterIdentity](suite.ctx, suite.T(), suite.state, []string{"cluster1-test"})

	assertNoResource[*omni.DiscoveryAuditResult](&suite.OmniSuite, omni.NewDiscoveryAuditResult(resources.DefaultNamespace, "cluster1-test"))

	cluster1Affiliates, err = discoveryClient.ListAffiliates(suite.ctx, "cluster1")
	suite.Require().NoError(err)

	suite.Empty(cluster1Affiliates)
}

type discoveryClientMock struct {
	affiliates map[string]map[string]struct{}
}

func (mock *discoveryClientMock) ListAffiliates(_ context.Context, cluster string) ([]string, error) {
	affiliates := maps.Keys(mock.affiliates[cluster])

	slices.Sort(affiliates)

	return affiliates, nil
}

func (mock *discoveryClientMock) DeleteAffiliate(_ context.Context, cluster, affiliate string) error {
	delete(mock.affiliates[cluster], affiliate)

	return nil
}

func (mock *discoveryClientMock) Close() error {
	return nil
}

func TestDiscoveryAuditSuite(t *testing.T) {
	suite.Run(t, new(DiscoveryAuditSuite))
}
