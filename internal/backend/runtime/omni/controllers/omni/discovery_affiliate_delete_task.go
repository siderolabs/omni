// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
)

// affiliateDeleteTimeout bounds each per-endpoint AffiliateDelete call independently of the reconcile context.
const affiliateDeleteTimeout = 30 * time.Second

// DiscoveryClientCache is an interface for interacting with discovery services.
type DiscoveryClientCache interface {
	AffiliateDelete(ctx context.Context, endpoint, cluster, affiliate string) error
}

// DiscoveryAffiliateDeleteTaskController manages DiscoveryAffiliateDeleteTask resource lifecycle.
//
// DiscoveryAffiliateDeleteTaskController generates cluster UUID for every cluster.
type DiscoveryAffiliateDeleteTaskController struct {
	discoveryClientCache DiscoveryClientCache
	affiliateTTL         time.Duration
}

// NewDiscoveryAffiliateDeleteTaskController creates a new DiscoveryAffiliateDeleteTaskController.
func NewDiscoveryAffiliateDeleteTaskController(discoveryClientCache DiscoveryClientCache) *DiscoveryAffiliateDeleteTaskController {
	return &DiscoveryAffiliateDeleteTaskController{
		discoveryClientCache: discoveryClientCache,
		affiliateTTL:         30 * time.Minute,
	}
}

// Name implements controller.QController interface.
func (ctrl *DiscoveryAffiliateDeleteTaskController) Name() string {
	return "DiscoveryAffiliateDeleteTaskController"
}

// Settings implements controller.QController interface.
func (ctrl *DiscoveryAffiliateDeleteTaskController) Settings() controller.QSettings {
	return controller.QSettings{
		Inputs: []controller.Input{
			{
				Namespace: resources.DefaultNamespace,
				Type:      omni.DiscoveryAffiliateDeleteTaskType,
				Kind:      controller.InputQPrimary,
			},
		},
		Outputs: []controller.Output{
			{
				Kind: controller.OutputShared,
				Type: omni.DiscoveryAffiliateDeleteTaskType,
			},
		},
		Concurrency: optional.Some[uint](4),
	}
}

// Reconcile implements controller.QController interface.
func (ctrl *DiscoveryAffiliateDeleteTaskController) Reconcile(ctx context.Context, logger *zap.Logger, r controller.QRuntime, ptr resource.Pointer) error {
	res, err := safe.ReaderGetByID[*omni.DiscoveryAffiliateDeleteTask](ctx, r, ptr.ID())
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil
		}

		return err
	}

	endpoints := res.TypedSpec().Value.DiscoveryServiceEndpoints
	if len(endpoints) == 0 && res.TypedSpec().Value.DiscoveryServiceEndpoint != "" {
		// fall back to the deprecated singular field for tasks written by the previous version
		endpoints = []string{res.TypedSpec().Value.DiscoveryServiceEndpoint}
	}

	expiredOnDiscoveryService := res.Metadata().Created().Add(ctrl.affiliateTTL).Before(time.Now())
	if expiredOnDiscoveryService {
		logger.Info(
			"skipping affiliate delete, already expired on discovery service",
			zap.String("cluster_id", res.TypedSpec().Value.ClusterId),
			zap.String("affiliate_id", res.Metadata().ID()),
			zap.Strings("endpoints", endpoints),
		)
	}

	isTearingDown := res.Metadata().Phase() == resource.PhaseTearingDown
	affiliateAlreadyDeleted := expiredOnDiscoveryService || isTearingDown

	if !affiliateAlreadyDeleted {
		clusterID := res.TypedSpec().Value.ClusterId
		affiliateID := res.Metadata().ID()
		deleteErrs := make([]error, len(endpoints))

		var wg sync.WaitGroup

		for i, endpoint := range endpoints {
			wg.Go(func() {
				callCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), affiliateDeleteTimeout)
				defer cancel()

				if deleteErr := ctrl.discoveryClientCache.AffiliateDelete(callCtx, endpoint, clusterID, affiliateID); deleteErr != nil {
					deleteErrs[i] = fmt.Errorf("error deleting affiliate %q/%q from %q: %w", clusterID, affiliateID, endpoint, deleteErr)

					return
				}

				logger.Info(
					"deleted the affiliate from the discovery service",
					zap.String("cluster_id", clusterID),
					zap.String("affiliate_id", affiliateID),
					zap.String("endpoint", endpoint),
				)
			})
		}

		wg.Wait()

		if err = errors.Join(deleteErrs...); err != nil {
			return err
		}
	}

	destroyReady, err := helpers.TeardownAndDestroy(ctx, r, ptr, controller.WithOwner(ClusterMachineTeardownControllerName))
	if err != nil {
		return err
	}

	if !destroyReady {
		return nil
	}

	logger.Info("destroyed DiscoveryAffiliateDeleteTask")

	return err
}

// MapInput implements controller.QController interface.
func (ctrl *DiscoveryAffiliateDeleteTaskController) MapInput(_ context.Context, _ *zap.Logger, _ controller.QRuntime, ptr controller.ReducedResourceMetadata) ([]resource.Pointer, error) {
	if ptr.Type() == omni.DiscoveryAffiliateDeleteTaskType {
		return []resource.Pointer{ptr}, nil
	}

	return nil, fmt.Errorf("unexpected resource type %q", ptr.Type())
}
