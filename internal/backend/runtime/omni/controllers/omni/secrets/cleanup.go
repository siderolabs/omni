// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package secrets

import (
	"context"
	"fmt"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
)

// ImportedClusterSecretsCleanupControllerName is the name of the controller.
const ImportedClusterSecretsCleanupControllerName = "ImportedClusterSecretsCleanupController"

// ImportedClusterSecretsCleanupController destroys ImportedClusterSecrets once the SecretsController
// has copied the secrets bundle into the matching ClusterSecrets and marked it as imported,
// so the source secrets do not linger in the state after they have been consumed.
type ImportedClusterSecretsCleanupController struct{}

// Name implements controller.QController interface.
func (ctrl *ImportedClusterSecretsCleanupController) Name() string {
	return ImportedClusterSecretsCleanupControllerName
}

// Settings implements controller.QController interface.
func (ctrl *ImportedClusterSecretsCleanupController) Settings() controller.QSettings {
	return controller.QSettings{
		Inputs: []controller.Input{
			{
				Namespace: resources.DefaultNamespace,
				Type:      omni.ClusterSecretsType,
				Kind:      controller.InputQPrimary,
			},
		},
		Outputs: []controller.Output{
			{
				Type: omni.ImportedClusterSecretsType,
				Kind: controller.OutputShared,
			},
		},
	}
}

// Reconcile implements controller.QController interface.
func (ctrl *ImportedClusterSecretsCleanupController) Reconcile(ctx context.Context, logger *zap.Logger, r controller.QRuntime, ptr resource.Pointer) error {
	cs, err := safe.ReaderGetByID[*omni.ClusterSecrets](ctx, r, ptr.ID())
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil
		}

		return fmt.Errorf("failed to get ClusterSecrets %q: %w", ptr.ID(), err)
	}

	if !cs.TypedSpec().Value.GetImported() {
		return nil
	}

	ics, err := safe.ReaderGetByID[*omni.ImportedClusterSecrets](ctx, r, cs.Metadata().ID())
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil
		}

		return fmt.Errorf("failed to get ImportedClusterSecrets %q: %w", cs.Metadata().ID(), err)
	}

	logger.Info("destroying consumed imported cluster secrets", zap.String("id", ics.Metadata().ID()))

	destroyed, err := helpers.TeardownAndDestroy(ctx, r, ics.Metadata(), controller.WithOwner(""))
	if err != nil {
		return fmt.Errorf("failed to teardown and destroy ImportedClusterSecrets %q: %w", ics.Metadata().ID(), err)
	}

	if !destroyed {
		return controller.NewRequeueError(fmt.Errorf("waiting for ImportedClusterSecrets %q to be destroyed", ics.Metadata().ID()), 10*time.Second)
	}

	return nil
}

// MapInput implements controller.QController interface.
func (ctrl *ImportedClusterSecretsCleanupController) MapInput(context.Context, *zap.Logger, controller.QRuntime, controller.ReducedResourceMetadata) ([]resource.Pointer, error) {
	return nil, nil
}
