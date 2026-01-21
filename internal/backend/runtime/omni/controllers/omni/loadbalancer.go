// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/loadbalancer"
)

// LoadBalancerController starts, stops and updates the statuses of load balancers.
type LoadBalancerController struct {
	NewLoadBalancer loadbalancer.NewFunc

	loadBalancers *loadbalancer.Manager
}

// Name returns the name of the controller.
func (ctrl *LoadBalancerController) Name() string {
	return "LoadBalancerController"
}

// Inputs implements controller.Controller interface.
func (ctrl *LoadBalancerController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: resources.DefaultNamespace,
			Type:      omni.LoadBalancerConfigType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: resources.DefaultNamespace,
			Type:      omni.ClusterStatusType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *LoadBalancerController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: omni.LoadBalancerStatusType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *LoadBalancerController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	if ctrl.NewLoadBalancer == nil {
		ctrl.NewLoadBalancer = loadbalancer.DefaultNew
	}

	ctrl.loadBalancers = loadbalancer.NewManager(logger, ctrl.NewLoadBalancer)

	defer func() {
		if err := ctrl.loadBalancers.Stop(); err != nil {
			logger.Error("error shutting down load balancers", zap.Error(err))
		}
	}()

	ticker := time.NewTicker(time.Second * 10)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
			if err := ctrl.reconcileLoadBalancers(ctx, r, logger); err != nil {
				return err
			}

			// reconcile health statuses immediately on lb config changes
			if err := ctrl.updateHealthStatus(ctx, r); err != nil {
				return err
			}
		case <-ticker.C:
			if err := ctrl.updateHealthStatus(ctx, r); err != nil {
				return err
			}
		}

		r.ResetRestartBackoff()
	}
}

func (ctrl *LoadBalancerController) reconcileLoadBalancers(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	// list loadBalancers
	list, err := safe.ReaderListAll[*omni.LoadBalancerConfig](ctx, r)
	if err != nil {
		return fmt.Errorf("error listing load balancer resources: %w", err)
	}

	// figure out which loadBalancers should run
	shouldRun := make(map[loadbalancer.ID]loadbalancer.Spec, list.Len())
	endpoints := make(map[loadbalancer.ID][]string, list.Len())
	stopped := map[loadbalancer.ID]struct{}{}

	for item := range list.All() {
		var clusterStatus *omni.ClusterStatus

		clusterStatus, err = safe.ReaderGet[*omni.ClusterStatus](ctx, r, omni.NewClusterStatus(
			item.Metadata().ID(),
		).Metadata())
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return err
		}

		if !clusterStatus.TypedSpec().Value.HasConnectedControlPlanes {
			logger.Debug("skip running load balancer for the cluster with no connected control plane machines", zap.String("cluster", item.Metadata().ID()))

			stopped[item.Metadata().ID()] = struct{}{}

			continue
		}

		lbSpec := item.TypedSpec().Value

		var bindPort int

		bindPort, err = strconv.Atoi(lbSpec.BindPort)
		if err != nil {
			return fmt.Errorf("failed to parse port %q as int: %w", lbSpec.BindPort, err)
		}

		shouldRun[item.Metadata().ID()] = loadbalancer.Spec{
			BindAddress: "0.0.0.0",
			BindPort:    bindPort,
		}

		endpoints[item.Metadata().ID()] = lbSpec.Endpoints
	}

	if err = ctrl.loadBalancers.Reconcile(shouldRun); err != nil {
		// don't fail hard to keep other loadbalancers running
		logger.Error("error reconciling load balancers", zap.Error(err))
	}

	ctrl.loadBalancers.UpdateUpstreams(endpoints)

	// Clean up statuses of load balancers that have been deleted or stopped.
	statuses, err := safe.ReaderListAll[*omni.LoadBalancerStatus](ctx, r)
	if err != nil {
		return fmt.Errorf("error listing resources: %w", err)
	}

	for cfg := range statuses.All() {
		id := cfg.Metadata().ID()

		if _, ok := stopped[id]; ok {
			if err := safe.WriterModify(ctx, r, omni.NewLoadBalancerStatus(id), func(status *omni.LoadBalancerStatus) error {
				status.Metadata().Labels().Set(omni.LabelCluster, id)

				status.TypedSpec().Value.Stopped = true
				status.TypedSpec().Value.Healthy = false

				return nil
			}); err != nil {
				return fmt.Errorf("error modifying load balancer resource: %w", err)
			}

			// keep the status
			continue
		}

		if _, ok := shouldRun[id]; ok {
			continue
		}

		if err := r.Destroy(ctx, cfg.Metadata()); err != nil {
			return fmt.Errorf("error deleting load balancer config '%s' error: %w", id, err)
		}
	}

	return nil
}

func (ctrl *LoadBalancerController) updateHealthStatus(ctx context.Context, r controller.Runtime) error {
	for id, healthy := range ctrl.loadBalancers.GetHealthStatus() {
		if err := safe.WriterModify(ctx, r, omni.NewLoadBalancerStatus(id), func(status *omni.LoadBalancerStatus) error {
			status.Metadata().Labels().Set(omni.LabelCluster, id)

			status.TypedSpec().Value.Stopped = false
			status.TypedSpec().Value.Healthy = healthy

			return nil
		}); err != nil {
			return fmt.Errorf("error modifying load balancer resource: %w", err)
		}
	}

	return nil
}
