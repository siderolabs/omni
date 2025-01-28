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
	"github.com/cosi-project/runtime/pkg/safe"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/auth"
)

// SAMLAssertionController manages SAMLAssertion resource lifecycle.
type SAMLAssertionController struct{}

// Name implements controller.Controller interface.
func (ctrl *SAMLAssertionController) Name() string {
	return "SAMLAssertionController"
}

// Inputs implements controller.Controller interface.
func (ctrl *SAMLAssertionController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Type:      auth.SAMLAssertionType,
			Kind:      controller.InputWeak,
			Namespace: resources.DefaultNamespace,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *SAMLAssertionController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: auth.SAMLAssertionType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *SAMLAssertionController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		case <-ticker.C:
		}

		sessions, err := safe.ReaderListAll[*auth.SAMLAssertion](ctx, r)
		if err != nil {
			return fmt.Errorf("error listing Cluster resources: %w", err)
		}

		for assertion := range sessions.All() {
			if time.Since(assertion.Metadata().Created()) > time.Hour*24 {
				err = r.Destroy(ctx, assertion.Metadata(), controller.WithOwner(""))
				if err != nil {
					return err
				}
			}
		}
	}
}
