// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"context"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
)

type KubernetesNodeAuditSuite struct {
	OmniSuite
}

func (suite *KubernetesNodeAuditSuite) TestReconcile() {
	suite.startRuntime()

	kubernetesClient := &kubernetesClientMock{}
	getKubernetesClient := func(context.Context, string) (omnictrl.KubernetesClient, error) {
		return kubernetesClient, nil
	}

	kubernetesNodeAuditController := omnictrl.NewKubernetesNodeAuditController(getKubernetesClient, 2*time.Second)

	suite.Require().NoError(suite.runtime.RegisterQController(kubernetesNodeAuditController))

	// create cluster1 that has a Kubernetes node mismatch

	cluster1KubernetesNodes := omni.NewClusterKubernetesNodes(resources.DefaultNamespace, "cluster1")
	cluster1KubernetesStatus := omni.NewKubernetesStatus(resources.DefaultNamespace, "cluster1")

	cluster1KubernetesNodes.TypedSpec().Value.Nodes = []string{"cluster1-node1", "cluster1-node2", "cluster1-node3"}
	cluster1KubernetesStatus.TypedSpec().Value.Nodes = []*specs.KubernetesStatusSpec_NodeStatus{
		{Nodename: "cluster1-node1"},
		{Nodename: "cluster1-node2"},
		{Nodename: "cluster1-node3"},
		{Nodename: "cluster1-node4"},
		{Nodename: "cluster1-node5", Ready: true},
	}

	// create cluster2 that has no Kubernetes node mismatch

	cluster2KubernetesNodes := omni.NewClusterKubernetesNodes(resources.DefaultNamespace, "cluster2")
	cluster2KubernetesStatus := omni.NewKubernetesStatus(resources.DefaultNamespace, "cluster2")

	cluster2KubernetesNodes.TypedSpec().Value.Nodes = []string{"cluster2-node1", "cluster2-node2", "cluster2-node3"}
	cluster2KubernetesStatus.TypedSpec().Value.Nodes = []*specs.KubernetesStatusSpec_NodeStatus{
		{Nodename: "cluster2-node1"},
		{Nodename: "cluster2-node2"},
		{Nodename: "cluster2-node3"},
	}

	suite.Require().NoError(suite.state.Create(suite.ctx, cluster1KubernetesNodes))
	suite.Require().NoError(suite.state.Create(suite.ctx, cluster2KubernetesNodes))
	suite.Require().NoError(suite.state.Create(suite.ctx, cluster1KubernetesStatus))
	suite.Require().NoError(suite.state.Create(suite.ctx, cluster2KubernetesStatus))

	// assert the audit results

	assertResource[*omni.KubernetesNodeAuditResult](&suite.OmniSuite, omni.NewKubernetesNodeAuditResult(resources.DefaultNamespace, "cluster1").Metadata(),
		func(res *omni.KubernetesNodeAuditResult, assertion *assert.Assertions) {
			assertion.Equal([]string{"cluster1-node4"}, res.TypedSpec().Value.DeletedNodes)
		})

	assertNoResource[*omni.KubernetesNodeAuditResult](&suite.OmniSuite, omni.NewKubernetesNodeAuditResult(resources.DefaultNamespace, "cluster2"))

	// assert the Kubernetes state

	require.ElementsMatch(suite.T(), []string{"cluster1-node4"}, kubernetesClient.deleted)

	// mark cluster1-node5 as not ready - auditor must delete it after this
	_, err := safe.StateUpdateWithConflicts[*omni.KubernetesStatus](suite.ctx, suite.state, cluster1KubernetesStatus.Metadata(), func(res *omni.KubernetesStatus) error {
		res.TypedSpec().Value.Nodes = []*specs.KubernetesStatusSpec_NodeStatus{
			{Nodename: "cluster1-node1"},
			{Nodename: "cluster1-node2"},
			{Nodename: "cluster1-node3"},
			{Nodename: "cluster1-node5", Ready: false},
		}

		return nil
	})
	suite.Require().NoError(err)

	// mark cluster2-node1 as the only valid node - cluster2-node2 and cluster2-node3 must be deleted after this

	_, err = safe.StateUpdateWithConflicts[*omni.ClusterKubernetesNodes](suite.ctx, suite.state, cluster2KubernetesNodes.Metadata(), func(res *omni.ClusterKubernetesNodes) error {
		res.TypedSpec().Value.Nodes = []string{"cluster2-node1"}

		return nil
	})
	suite.Require().NoError(err)

	// assert the audit results

	assertResource[*omni.KubernetesNodeAuditResult](&suite.OmniSuite, omni.NewKubernetesNodeAuditResult(resources.DefaultNamespace, "cluster1").Metadata(),
		func(res *omni.KubernetesNodeAuditResult, assertion *assert.Assertions) {
			assertion.Equal([]string{"cluster1-node5"}, res.TypedSpec().Value.DeletedNodes)
		})

	assertResource[*omni.KubernetesNodeAuditResult](&suite.OmniSuite, omni.NewKubernetesNodeAuditResult(resources.DefaultNamespace, "cluster2").Metadata(),
		func(res *omni.KubernetesNodeAuditResult, assertion *assert.Assertions) {
			assertion.Equal([]string{"cluster2-node2", "cluster2-node3"}, res.TypedSpec().Value.DeletedNodes)
		})

	// assert the Kubernetes state

	require.ElementsMatch(suite.T(), []string{"cluster1-node4", "cluster1-node5", "cluster2-node2", "cluster2-node3"}, kubernetesClient.deleted)
}

type kubernetesClientMock struct {
	deleted []string
}

func (k *kubernetesClientMock) DeleteNode(_ context.Context, node string) error {
	k.deleted = append(k.deleted, node)

	return nil
}

func TestKubernetesNodeAuditSuite(t *testing.T) {
	suite.Run(t, new(KubernetesNodeAuditSuite))
}
