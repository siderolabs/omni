// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xerrors"
	"github.com/siderolabs/gen/xslices"
	"go.uber.org/zap"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime"
	"github.com/siderolabs/omni/internal/backend/runtime/kubernetes"
)

// KubernetesClient is an interface that defines the methods to interact with the Kubernetes cluster.
type KubernetesClient interface {
	DeleteNode(ctx context.Context, node string) error
}

// GetKubernetesClientFunc is a function that returns a KubernetesClient for the given cluster.
type GetKubernetesClientFunc func(ctx context.Context, cluster string) (KubernetesClient, error)

// KubernetesNodeAuditController creates the system config patch that contains the maintenance config.
type KubernetesNodeAuditController = qtransform.QController[*omni.ClusterKubernetesNodes, *omni.KubernetesNodeAuditResult]

// NewKubernetesNodeAuditController initializes KubernetesNodeAuditController.
func NewKubernetesNodeAuditController(getKubernetesClientFunc GetKubernetesClientFunc, deleteOlderThan time.Duration) *KubernetesNodeAuditController {
	requeueAfter := deleteOlderThan + time.Second
	auditor := &nodeAuditor{
		deleteOlderThan: deleteOlderThan,
		requeueAfter:    requeueAfter,
	}

	if getKubernetesClientFunc == nil {
		getKubernetesClientFunc = getKubernetesClient
	}

	return qtransform.NewQController(
		qtransform.Settings[*omni.ClusterKubernetesNodes, *omni.KubernetesNodeAuditResult]{
			Name: "KubernetesNodeAuditController",
			MapMetadataFunc: func(nodes *omni.ClusterKubernetesNodes) *omni.KubernetesNodeAuditResult {
				return omni.NewKubernetesNodeAuditResult(resources.DefaultNamespace, nodes.Metadata().ID())
			},
			UnmapMetadataFunc: func(result *omni.KubernetesNodeAuditResult) *omni.ClusterKubernetesNodes {
				return omni.NewClusterKubernetesNodes(resources.DefaultNamespace, result.Metadata().ID())
			},
			TransformFunc: func(ctx context.Context, r controller.Reader, logger *zap.Logger, nodes *omni.ClusterKubernetesNodes, result *omni.KubernetesNodeAuditResult) error {
				status, err := safe.ReaderGetByID[*omni.KubernetesStatus](ctx, r, nodes.Metadata().ID())
				if err != nil {
					if state.IsNotFoundError(err) {
						return nil
					}

					return fmt.Errorf("failed to get kubernetes status: %w", err)
				}

				cluster := nodes.Metadata().ID()
				actualNodes := xslices.Map(status.TypedSpec().Value.GetNodes(), func(nodeStatus *specs.KubernetesStatusSpec_NodeStatus) string {
					return nodeStatus.Nodename
				})
				desiredNodes := nodes.TypedSpec().Value.Nodes

				if slices.Contains(desiredNodes, "") {
					logger.Warn("found a node with an empty name in the list of Kubernetes nodes, skip", zap.Strings("nodes", desiredNodes))

					return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("found empty node in the desired nodes")
				}

				pending, readyToDelete := auditor.calculateNodesStatus(cluster, desiredNodes, actualNodes)

				logger = logger.With(zap.Strings("pending", pending), zap.Strings("ready_to_delete", readyToDelete))

				if len(readyToDelete) == 0 {
					if len(pending) > 0 {
						logger.Info("there are invalid nodes that are not ready to be deleted yet, requeue the audit")

						// there are invalid nodes that are not ready to be deleted yet - requeue the audit
						return controller.NewRequeueInterval(requeueAfter)
					}

					logger.Debug("no nodes are ready to be deleted in this run, skip updating the result")

					return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("no nodes to delete, skip updating the result")
				}

				logger.Info("found nodes ready to be deleted")

				deleted, requeue, err := auditor.deleteNodes(ctx, logger, status, getKubernetesClientFunc, readyToDelete)
				if err != nil {
					return err
				}

				result.TypedSpec().Value.DeletedNodes = deleted

				if requeue {
					logger.Warn("not all nodes could be deleted, requeue the audit", zap.Strings("deleted_nodes", deleted))

					return controller.NewRequeueInterval(requeueAfter)
				}

				if len(pending) > 0 {
					logger.Info("there are still nodes that are not ready to be deleted yet, requeue the audit")

					return controller.NewRequeueInterval(requeueAfter)
				}

				return nil
			},
		},
		qtransform.WithConcurrency(2),
		qtransform.WithExtraMappedInput(qtransform.MapperSameID[*omni.KubernetesStatus, *omni.ClusterKubernetesNodes]()),
	)
}

// nodeAuditor is responsible for keeping track and auditing of the nodes in the Kubernetes clusters.
type nodeAuditor struct {
	// clusterToInvalidNodes maps a cluster ID to a map of invalid node IDs to the time they were marked as invalid.
	//
	// clusterID -> nodeID -> invalidSince
	clusterToInvalidNodes map[string]map[string]time.Time

	deleteOlderThan time.Duration
	requeueAfter    time.Duration

	lock sync.Mutex
}

// calculateNodesStatus calculates the status of the nodes in the given cluster.
//
// It returns:
// - the IDs of the nodes that are "pending" - i.e., they are invalid, but not ready to be deleted yet because they have not been invalid for long enough.
// - the IDs of the nodes that are "ready to delete" - i.e., they are invalid and have been invalid for long enough to be deleted.
func (auditor *nodeAuditor) calculateNodesStatus(cluster string, desiredNodes, actualNodes []string) (pending, readyToDelete []string) {
	invalidNodes := xslices.ToSet(actualNodes)

	for _, localNode := range desiredNodes {
		delete(invalidNodes, localNode)
	}

	auditor.lock.Lock()
	defer auditor.lock.Unlock()

	if len(invalidNodes) == 0 {
		delete(auditor.clusterToInvalidNodes, cluster)

		return nil, nil
	}

	if auditor.clusterToInvalidNodes == nil {
		auditor.clusterToInvalidNodes = make(map[string]map[string]time.Time)
	}

	if auditor.clusterToInvalidNodes[cluster] == nil {
		auditor.clusterToInvalidNodes[cluster] = make(map[string]time.Time, len(invalidNodes))
	}

	currentInvalidNodes := auditor.clusterToInvalidNodes[cluster]

	// clear the invalid nodes that are no longer invalid
	for node := range currentInvalidNodes {
		if _, ok := invalidNodes[node]; !ok {
			delete(currentInvalidNodes, node)
		}
	}

	// mark the invalid nodes
	// if the node was already marked as invalid, we don't update its invalidSince timestamp
	for node := range invalidNodes {
		if _, ok := currentInvalidNodes[node]; !ok {
			currentInvalidNodes[node] = time.Now()
		}
	}

	if len(currentInvalidNodes) == 0 {
		return nil, nil
	}

	for node, invalidSince := range currentInvalidNodes {
		if time.Since(invalidSince) > auditor.deleteOlderThan {
			readyToDelete = append(readyToDelete, node)

			continue
		}

		// this is an invalid node, but it is not ready to be deleted yet, mark the audit to be re-queued
		pending = append(pending, node)
	}

	slices.Sort(pending)
	slices.Sort(readyToDelete)

	return pending, readyToDelete
}

// deleteNodes deletes the given nodes from the Kubernetes cluster.
func (auditor *nodeAuditor) deleteNodes(ctx context.Context, logger *zap.Logger,
	status *omni.KubernetesStatus, getKubernetesClientFunc GetKubernetesClientFunc, nodesToDelete []string,
) (deleted []string, requeue bool, err error) {
	if len(nodesToDelete) == 0 {
		return nil, false, nil
	}

	cluster := status.Metadata().ID()
	nodeToReadyStatus := make(map[string]bool)

	for _, nodeStatus := range status.TypedSpec().Value.GetNodes() {
		nodeToReadyStatus[nodeStatus.Nodename] = nodeStatus.Ready
	}

	kubeClient, err := getKubernetesClientFunc(ctx, cluster)
	if err != nil {
		return nil, false, fmt.Errorf("failed to get kubernetes client: %w", err)
	}

	deleted = make([]string, 0, len(nodesToDelete))

	for _, node := range nodesToDelete {
		if isReady := nodeToReadyStatus[node]; isReady {
			logger.Warn("node is marked as ready to be deleted but still reporting Ready in Kubernetes, skipping", zap.String("node", node))

			continue
		}

		if err = kubeClient.DeleteNode(ctx, node); err != nil {
			logger.Error("failed to delete node", zap.String("node", node), zap.Error(err))

			continue
		}

		deleted = append(deleted, node)
	}

	if len(deleted) == 0 {
		// no nodes could be deleted in this run - skip updating the result and requeue with an error
		return nil, false, controller.NewRequeueErrorf(auditor.requeueAfter, "failed to delete any of the nodes (%q) for cluster %q",
			nodesToDelete, cluster)
	}

	auditor.unmarkNodesAsInvalid(cluster, deleted)

	if len(deleted) < len(nodesToDelete) {
		return deleted, true, nil
	}

	logger.Info("deleted nodes successfully", zap.Strings("deleted_nodes", deleted))

	return deleted, false, nil
}

// unmarkNodesAsInvalid clears the marked "invalid" status for the node IDs in the given cluster.
func (auditor *nodeAuditor) unmarkNodesAsInvalid(cluster string, nodes []string) {
	auditor.lock.Lock()
	defer auditor.lock.Unlock()

	if auditor.clusterToInvalidNodes == nil {
		return
	}

	invalidNodes := auditor.clusterToInvalidNodes[cluster]

	for _, node := range nodes {
		delete(invalidNodes, node)
	}

	if len(invalidNodes) == 0 {
		delete(auditor.clusterToInvalidNodes, cluster)
	}
}

func getKubernetesClient(ctx context.Context, cluster string) (KubernetesClient, error) {
	type kubeRuntime interface {
		GetClient(ctx context.Context, cluster string) (*kubernetes.Client, error)
	}

	k8s, err := runtime.LookupInterface[kubeRuntime](kubernetes.Name)
	if err != nil {
		return nil, err
	}

	k8sClient, err := k8s.GetClient(ctx, cluster)
	if err != nil {
		return nil, fmt.Errorf("error getting kubernetes client for cluster %q: %w", cluster, err)
	}

	return &kubernetesClient{
		client: k8sClient,
	}, nil
}

type kubernetesClient struct {
	client *kubernetes.Client
}

func (cli *kubernetesClient) DeleteNode(ctx context.Context, node string) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	err := cli.client.Clientset().CoreV1().Nodes().Delete(ctx, node, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("error deleting node %q: %w", node, err)
	}

	return nil
}
