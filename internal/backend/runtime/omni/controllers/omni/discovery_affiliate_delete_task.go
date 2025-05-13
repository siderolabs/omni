// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"fmt"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/jonboulle/clockwork"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
)

// DiscoveryClientCache is an interface for interacting with discovery services.
type DiscoveryClientCache interface {
	AffiliateDelete(ctx context.Context, endpoint, cluster, affiliate string) error
}

// DiscoveryAffiliateDeleteTaskController manages DiscoveryAffiliateDeleteTask resource lifecycle.
//
// DiscoveryAffiliateDeleteTaskController generates cluster UUID for every cluster.
type DiscoveryAffiliateDeleteTaskController struct {
	discoveryClientCache DiscoveryClientCache
	clock                clockwork.Clock
	affiliateTTL         time.Duration
}

// NewDiscoveryAffiliateDeleteTaskController creates a new DiscoveryAffiliateDeleteTaskController.
func NewDiscoveryAffiliateDeleteTaskController(clock clockwork.Clock, discoveryClientCache DiscoveryClientCache) *DiscoveryAffiliateDeleteTaskController {
	if clock == nil {
		clock = clockwork.NewRealClock()
	}

	return &DiscoveryAffiliateDeleteTaskController{
		clock:                clock,
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

	expiredOnDiscoveryService := res.Metadata().Created().Add(ctrl.affiliateTTL).Before(ctrl.clock.Now())
	if expiredOnDiscoveryService {
		logger.Info("skipping affiliate delete, already expired on discovery service",
			zap.String("cluster_id", res.TypedSpec().Value.ClusterId),
			zap.String("affiliate_id", res.Metadata().ID()),
			zap.String("endpoint", res.TypedSpec().Value.DiscoveryServiceEndpoint),
		)
	}

	isTearingDown := res.Metadata().Phase() == resource.PhaseTearingDown
	affiliateAlreadyDeleted := expiredOnDiscoveryService || isTearingDown

	if !affiliateAlreadyDeleted {
		endpoint := res.TypedSpec().Value.DiscoveryServiceEndpoint
		clusterID := res.TypedSpec().Value.ClusterId
		affiliateID := res.Metadata().ID()

		if err = ctrl.discoveryClientCache.AffiliateDelete(ctx, endpoint, clusterID, affiliateID); err != nil {
			return fmt.Errorf("error deleting affiliate %q/%q from %q: %w", clusterID, affiliateID, endpoint, err)
		}

		logger.Info("deleted the affiliate from the discovery service",
			zap.String("cluster_id", clusterID),
			zap.String("affiliate_id", affiliateID),
			zap.String("endpoint", endpoint),
		)
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
func (ctrl *DiscoveryAffiliateDeleteTaskController) MapInput(_ context.Context, _ *zap.Logger, _ controller.QRuntime, ptr resource.Pointer) ([]resource.Pointer, error) {
	if ptr.Type() == omni.DiscoveryAffiliateDeleteTaskType {
		return []resource.Pointer{ptr}, nil
	}

	return nil, fmt.Errorf("unexpected resource type %q", ptr.Type())
}
