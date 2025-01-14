// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
)

// ProviderHealthStatusOptions defines options for the provider health status controller.
type ProviderHealthStatusOptions struct {
	HealthCheckFunc func(ctx context.Context) error
	Interval        time.Duration
}

// NewProviderHealthStatusController creates a new provider health status controller.
func NewProviderHealthStatusController(providerID string, options ProviderHealthStatusOptions) (*ProviderHealthStatusController, error) {
	if providerID == "" {
		return nil, fmt.Errorf("providerID is required")
	}

	return &ProviderHealthStatusController{
		providerID: providerID,
		options:    options,
	}, nil
}

// ProviderHealthStatusController reports infra provider health status.
type ProviderHealthStatusController struct {
	providerID string
	options    ProviderHealthStatusOptions
}

// Name implements controller.Controller interface.
func (ctrl *ProviderHealthStatusController) Name() string {
	return ctrl.providerID + ".ProviderHealthStatusController"
}

// Inputs implements controller.Controller interface.
func (ctrl *ProviderHealthStatusController) Inputs() []controller.Input {
	return nil
}

// Outputs implements controller.Controller interface.
func (ctrl *ProviderHealthStatusController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: infra.InfraProviderHealthStatusType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *ProviderHealthStatusController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	interval := ctrl.options.Interval
	if interval == 0 {
		interval = 30 * time.Second
	}

	ticker := time.NewTicker(interval)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		case <-r.EventCh():
		}

		healthErrStr := ""

		if ctrl.options.HealthCheckFunc != nil {
			if healthErr := ctrl.options.HealthCheckFunc(ctx); healthErr != nil {
				healthErrStr = healthErr.Error()
			}
		}

		healthStatus := infra.NewProviderHealthStatus(ctrl.providerID)

		if err := safe.WriterModify(ctx, r, healthStatus, func(status *infra.ProviderHealthStatus) error {
			status.TypedSpec().Value.LastHeartbeatTimestamp = timestamppb.Now()
			status.TypedSpec().Value.Error = healthErrStr

			return nil
		}); err != nil {
			return err
		}

		r.ResetRestartBackoff()
	}
}
