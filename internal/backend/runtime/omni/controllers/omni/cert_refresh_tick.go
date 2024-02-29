// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/system"
)

// CertRefreshTickController creates ticks to notify all certificate management controllers to refresh certificates.
type CertRefreshTickController struct {
	tickInterval time.Duration
}

// NewCertRefreshTickController creates a new CertRefreshTickController.
func NewCertRefreshTickController(tickInterval time.Duration) *CertRefreshTickController {
	return &CertRefreshTickController{tickInterval: tickInterval}
}

// Name implements controller.Controller interface.
func (ctrl *CertRefreshTickController) Name() string {
	return "CertRefreshTickController"
}

// Inputs implements controller.Controller interface.
func (ctrl *CertRefreshTickController) Inputs() []controller.Input {
	return nil
}

// Outputs implements controller.Controller interface.
func (ctrl *CertRefreshTickController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: system.CertRefreshTickType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *CertRefreshTickController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	ticker := time.NewTicker(ctrl.tickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case t := <-ticker.C:
			// issue a "refresh" tick to notify all certificate management controllers to refresh certificates
			if err := safe.WriterModify(
				ctx, r,
				system.NewCertRefreshTick(resources.EphemeralNamespace, t.String()),
				func(*system.CertRefreshTick) error {
					return nil
				},
			); err != nil {
				return err
			}
		case <-r.EventCh(): // ignore, we have no inputs
		}
	}
}
