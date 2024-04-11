// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"fmt"
	"io"
	"slices"
	"sync"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/siderolabs/gen/xerrors"
	"github.com/siderolabs/gen/xslices"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/discovery"
)

// DiscoveryClient is the interface for the discovery service client.
type DiscoveryClient interface {
	ListAffiliates(ctx context.Context, cluster string) ([]string, error)
	DeleteAffiliate(ctx context.Context, cluster, affiliate string) error
	io.Closer
}

// CreateDiscoveryClientFunc is a function that creates a new DiscoveryClient.
type CreateDiscoveryClientFunc = func(ctx context.Context) (DiscoveryClient, error)

// DiscoveryAuditController creates the system config patch that contains the maintenance config.
type DiscoveryAuditController = qtransform.QController[*omni.ClusterIdentity, *omni.DiscoveryAuditResult]

// NewDiscoveryAuditController initializes DiscoveryAuditController.
func NewDiscoveryAuditController(createDiscoveryClient CreateDiscoveryClientFunc, deleteOlderThan time.Duration) *DiscoveryAuditController {
	requeueAfter := deleteOlderThan + 1*time.Second
	auditor := &discoveryAuditor{
		deleteOlderThan: deleteOlderThan,
		requeueAfter:    requeueAfter,
	}

	if createDiscoveryClient == nil {
		createDiscoveryClient = func(ctx context.Context) (DiscoveryClient, error) {
			return discovery.NewClient(ctx)
		}
	}

	return qtransform.NewQController(
		qtransform.Settings[*omni.ClusterIdentity, *omni.DiscoveryAuditResult]{
			Name: "DiscoveryAuditController",
			MapMetadataFunc: func(clusterIdentity *omni.ClusterIdentity) *omni.DiscoveryAuditResult {
				return omni.NewDiscoveryAuditResult(resources.DefaultNamespace, clusterIdentity.Metadata().ID())
			},
			UnmapMetadataFunc: func(discoveryAuditResult *omni.DiscoveryAuditResult) *omni.ClusterIdentity {
				return omni.NewClusterIdentity(resources.DefaultNamespace, discoveryAuditResult.Metadata().ID())
			},
			TransformFunc: func(ctx context.Context, _ controller.Reader, logger *zap.Logger, clusterIdentity *omni.ClusterIdentity, discoveryAuditResult *omni.DiscoveryAuditResult) error {
				discoveryClient, err := createDiscoveryClient(ctx)
				if err != nil {
					return fmt.Errorf("failed to create discovery client: %w", err)
				}

				defer func() {
					if closeErr := discoveryClient.Close(); closeErr != nil {
						logger.Error("failed to close discovery client", zap.Error(closeErr))
					}
				}()

				cluster := clusterIdentity.TypedSpec().Value.ClusterId

				discoveryAuditResult.TypedSpec().Value.ClusterId = cluster

				logger = logger.With(zap.String("cluster_id", cluster))

				affiliatesInRemote, err := discoveryClient.ListAffiliates(ctx, cluster)
				if err != nil {
					return err
				}

				localAffiliates := clusterIdentity.TypedSpec().Value.NodeIds
				pending, readyToDelete := auditor.calculateAffiliatesStatus(cluster, localAffiliates, affiliatesInRemote)

				logger = logger.With(zap.Strings("pending", pending), zap.Strings("ready_to_delete", readyToDelete))

				if len(readyToDelete) == 0 {
					if len(pending) > 0 {
						logger.Info("there are invalid affiliates that are not ready to be deleted yet, requeue the audit")

						// there are invalid affiliates that are not ready to be deleted yet - requeue the audit
						return controller.NewRequeueInterval(requeueAfter)
					}

					logger.Debug("no affiliates are ready to be deleted in this run, skip updating the result")

					return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("no affiliates to delete, skip updating the result")
				}

				logger.Info("found affiliates ready to be deleted")

				deleted, requeue, err := auditor.deleteAffiliates(ctx, discoveryClient, logger, clusterIdentity, cluster, readyToDelete)
				if err != nil {
					return err
				}

				discoveryAuditResult.TypedSpec().Value.DeletedAffiliateIds = deleted

				if requeue {
					logger.Warn("not all affiliates could be deleted, requeue the audit", zap.Strings("deleted_affiliates", deleted))

					return controller.NewRequeueInterval(requeueAfter)
				}

				if len(pending) > 0 {
					logger.Info("there are still affiliates that are not ready to be deleted yet, requeue the audit")

					return controller.NewRequeueInterval(requeueAfter)
				}

				return nil
			},
			FinalizerRemovalFunc: func(ctx context.Context, _ controller.Reader, logger *zap.Logger, clusterIdentity *omni.ClusterIdentity) error {
				discoveryClient, err := createDiscoveryClient(ctx)
				if err != nil {
					return fmt.Errorf("failed to create discovery client: %w", err)
				}

				defer func() {
					if closeErr := discoveryClient.Close(); closeErr != nil {
						logger.Error("failed to close discovery client", zap.Error(closeErr))
					}
				}()

				cluster := clusterIdentity.TypedSpec().Value.ClusterId

				logger = logger.With(zap.String("cluster_id", cluster))

				logger.Info("delete all affiliates for the tearing down cluster")

				affiliatesInRemote, err := discoveryClient.ListAffiliates(ctx, cluster)
				if err != nil {
					return err
				}

				deleted, requeue, err := auditor.deleteAffiliates(ctx, discoveryClient, logger, clusterIdentity, cluster, affiliatesInRemote)
				if err != nil {
					return err
				}

				if requeue {
					logger.Warn("not all affiliates could be deleted, requeue the audit", zap.Strings("deleted_affiliates", deleted))

					return controller.NewRequeueErrorf(auditor.requeueAfter, "failed to delete all affiliates for cluster %q(%q)", clusterIdentity.Metadata().ID(), cluster)
				}

				return nil
			},
		},
		qtransform.WithConcurrency(2),
	)
}

// discoveryAuditor is responsible for keeping track and auditing of the affiliates in the clusters.
// The "cluster" it accepts is *NOT* the ID of a cluster in Omni, it needs to be the ID of the cluster in the discovery service.
// The affiliates it accepts are the IDs of the nodes in the discovery service.
type discoveryAuditor struct {
	// clusterToInvalidAffiliates maps a cluster ID to a map of invalid affiliate IDs to the time they were marked as invalid.
	//
	// clusterID -> affiliateID -> invalidSince
	clusterToInvalidAffiliates map[string]map[string]time.Time

	deleteOlderThan time.Duration
	requeueAfter    time.Duration

	lock sync.Mutex
}

// calculateAffiliatesStatus calculates the status of the affiliates in the given cluster.
//
// It returns:
// - the IDs of the affiliates that are "pending" - i.e., they are invalid, but not ready to be deleted yet because they have not been invalid for long enough.
// - the IDs of the affiliates that are "ready to delete" - i.e., they are invalid and have been invalid for long enough to be deleted.
func (auditor *discoveryAuditor) calculateAffiliatesStatus(cluster string, localAffiliates, remoteAffiliates []string) (pending, readyToDelete []string) {
	invalidAffiliates := xslices.ToSet(remoteAffiliates)

	for _, localAffiliate := range localAffiliates {
		delete(invalidAffiliates, localAffiliate)
	}

	auditor.lock.Lock()
	defer auditor.lock.Unlock()

	if len(invalidAffiliates) == 0 {
		delete(auditor.clusterToInvalidAffiliates, cluster)

		return nil, nil
	}

	if auditor.clusterToInvalidAffiliates == nil {
		auditor.clusterToInvalidAffiliates = make(map[string]map[string]time.Time)
	}

	if auditor.clusterToInvalidAffiliates[cluster] == nil {
		auditor.clusterToInvalidAffiliates[cluster] = make(map[string]time.Time, len(invalidAffiliates))
	}

	currentInvalidAffiliates := auditor.clusterToInvalidAffiliates[cluster]

	// clear the invalid affiliates that are no longer invalid
	for affiliate := range currentInvalidAffiliates {
		if _, ok := invalidAffiliates[affiliate]; !ok {
			delete(currentInvalidAffiliates, affiliate)
		}
	}

	// mark the invalid affiliates
	// if the affiliate was already marked as invalid, we don't update its invalidSince timestamp
	for affiliate := range invalidAffiliates {
		if _, ok := currentInvalidAffiliates[affiliate]; !ok {
			currentInvalidAffiliates[affiliate] = time.Now()
		}
	}

	if len(currentInvalidAffiliates) == 0 {
		return nil, nil
	}

	for affiliate, invalidSince := range currentInvalidAffiliates {
		if time.Since(invalidSince) > auditor.deleteOlderThan {
			readyToDelete = append(readyToDelete, affiliate)

			continue
		}

		// this is an invalid affiliate, but it is not ready to be deleted yet, mark the audit to be re-queued
		pending = append(pending, affiliate)
	}

	slices.Sort(pending)
	slices.Sort(readyToDelete)

	return pending, readyToDelete
}

// deleteAffiliates deletes the affiliates with the given IDs in the discovery service for the given cluster.
func (auditor *discoveryAuditor) deleteAffiliates(ctx context.Context, discoveryClient DiscoveryClient, logger *zap.Logger,
	clusterIdentity *omni.ClusterIdentity, cluster string, affiliatesToDelete []string,
) (deleted []string, requeue bool, err error) {
	if len(affiliatesToDelete) == 0 {
		return nil, false, nil
	}

	deleted = make([]string, 0, len(affiliatesToDelete))

	for _, affiliate := range affiliatesToDelete {
		if err = discoveryClient.DeleteAffiliate(ctx, cluster, affiliate); err != nil {
			logger.Error("failed to delete affiliate", zap.String("affiliate", affiliate), zap.Error(err))
		}

		deleted = append(deleted, affiliate)
	}

	if len(deleted) == 0 {
		// no affiliates could be deleted in this run - skip updating the result and requeue with an error
		return nil, false, controller.NewRequeueErrorf(auditor.requeueAfter, "failed to delete any of the affiliates (%q) for cluster %q(%q)",
			affiliatesToDelete, clusterIdentity.Metadata().ID(), cluster)
	}

	auditor.unmarkAffiliatesAsInvalid(cluster, deleted)

	if len(deleted) < len(affiliatesToDelete) {
		return deleted, true, nil
	}

	logger.Info("deleted affiliates successfully", zap.Strings("deleted_affiliates", deleted))

	return deleted, false, nil
}

// unmarkAffiliatesAsInvalid clears the marked "invalid" status for the affiliate IDs in the given cluster.
func (auditor *discoveryAuditor) unmarkAffiliatesAsInvalid(cluster string, affiliates []string) {
	auditor.lock.Lock()
	defer auditor.lock.Unlock()

	if auditor.clusterToInvalidAffiliates == nil {
		return
	}

	invalidAffiliates := auditor.clusterToInvalidAffiliates[cluster]

	for _, affiliate := range affiliates {
		delete(invalidAffiliates, affiliate)
	}

	if len(invalidAffiliates) == 0 {
		delete(auditor.clusterToInvalidAffiliates, cluster)
	}
}
