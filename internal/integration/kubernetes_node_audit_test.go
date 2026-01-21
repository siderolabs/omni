// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build integration

package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/integration/kubernetes"
)

// AssertKubernetesNodeAudit tests the Kubernetes node audit feature (KubernetesNodeAuditController) by doing the following:
//  1. Freeze the whole control plane, so that kube-apiserver won't be accessible to the ClusterMachineTeardownController,
//     so it won't be able to remove the node from Kubernetes at the moment of the node deletion.
//  2. Freeze & force-delete a worker node. It won't be removed from Kubernetes due to the control plane being frozen.
//  3. Assert that the ClusterMachine resource is deleted - the ClusterMachineTeardownController did not block its deletion despite failing to remove the node from Kubernetes.
//  4. Wake the control plane back up.
//  5. Assert that the worker node eventually gets removed from Kubernetes due to node audit.
func AssertKubernetesNodeAudit(testCtx context.Context, clusterName string, options *TestOptions) TestFunc {
	st := options.omniClient.Omni().State()
	return func(t *testing.T) {
		ctx := kubernetes.WrapContext(testCtx, t)

		if options.FreezeAMachineFunc == nil || options.RestartAMachineFunc == nil {
			t.Skip("skip the test as FreezeAMachineFunc or RestartAMachineFunc is not set")
		}

		logger := zaptest.NewLogger(t)

		cpIDs := rtestutils.ResourceIDs[*omni.MachineSetNode](ctx, t, st, state.WithLabelQuery(
			resource.LabelEqual(omni.LabelCluster, clusterName),
			resource.LabelExists(omni.LabelControlPlaneRole),
		))
		require.NotEmpty(t, cpIDs, "no control plane nodes found")

		workerIDs := rtestutils.ResourceIDs[*omni.MachineSetNode](ctx, t, st, state.WithLabelQuery(
			resource.LabelEqual(omni.LabelCluster, clusterName),
			resource.LabelExists(omni.LabelWorkerRole),
		))
		require.NotEmpty(t, workerIDs, "no worker nodes found")

		logger.Info("freeze control plane")

		freezeMachinesOfType(ctx, t, st, clusterName, options.FreezeAMachineFunc, omni.LabelControlPlaneRole)

		workerID := workerIDs[0]

		workerIdentity, err := safe.StateGetByID[*omni.ClusterMachineIdentity](ctx, st, workerID)
		require.NoError(t, err)

		workerNodeName := workerIdentity.TypedSpec().Value.Nodename

		logger.Info("freeze the worker node", zap.String("id", workerID))

		err = options.FreezeAMachineFunc(ctx, workerID)
		require.NoError(t, err)

		logger.Info("force delete & wipe the worker node", zap.String("id", workerID))

		wipeMachine(ctx, t, st, workerID, options.WipeAMachineFunc)

		// assert that the ClusterMachine is deleted.
		// here, the ClusterMachineTeardownController will fail to remove the node from Kubernetes, as the control plane is frozen.
		// but it should not block the deletion of the ClusterMachine resource.
		rtestutils.AssertNoResource[*omni.ClusterMachine](ctx, t, st, workerID)

		logger.Info("wake the control plane back up")

		for _, id := range cpIDs {
			require.NoError(t, options.RestartAMachineFunc(ctx, id))
		}

		kubernetesClient := kubernetes.GetClient(ctx, t, options.omniClient.Management(), clusterName)

		logger.Info("assert that the node is removed from Kubernetes due to node audit")

		count := 0

		require.EventuallyWithT(t, func(collect *assert.CollectT) {
			require.NoError(collect, ctx.Err()) // if the context is done, fail immediately

			count++
			log := count%6 == 0 // log at most once every 30 seconds

			if log {
				logger.Info("list nodes in Kubernetes to check if the worker node is removed")
			}

			nodeList, listErr := kubernetesClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
			if !assert.NoError(collect, listErr) && log {
				logger.Error("failed to list nodes in Kubernetes", zap.Error(listErr))
			}

			nodeNames := make([]string, 0, len(nodeList.Items))

			for _, k8sNode := range nodeList.Items {
				nodeNames = append(nodeNames, k8sNode.Name)
			}

			if !assert.NotContains(collect, nodeNames, workerNodeName, "worker node should not be present in the list of nodes in Kubernetes") && log {
				logger.Error("worker node is still present in the list of nodes in Kubernetes", zap.String("node", workerNodeName), zap.Strings("nodes", nodeNames))
			}
		}, 10*time.Minute, 5*time.Second)
	}
}
