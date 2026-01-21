// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"fmt"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/cosi/helpers"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
)

// NodeUniqueTokenCleanupControllerName is the name of the NodeUniqueTokenCleanupController.
const NodeUniqueTokenCleanupControllerName = "NodeUniqueTokenCleanupController"

// NodeUniqueTokenCleanupController deletes orphaned NodeUniqueToken resources.
type NodeUniqueTokenCleanupController struct {
	generic.NamedController
	orphanTTL time.Duration
}

// NewNodeUniqueTokenCleanupController initializes NodeUniqueTokenCleanupController.
func NewNodeUniqueTokenCleanupController(orphanTTL time.Duration) *NodeUniqueTokenCleanupController {
	return &NodeUniqueTokenCleanupController{
		NamedController: generic.NamedController{
			ControllerName: NodeUniqueTokenCleanupControllerName,
		},
		orphanTTL: orphanTTL,
	}
}

// Settings implements controller.QController interface.
func (ctrl *NodeUniqueTokenCleanupController) Settings() controller.QSettings {
	return controller.QSettings{
		Inputs: []controller.Input{
			{
				Namespace: resources.DefaultNamespace,
				Type:      siderolink.NodeUniqueTokenType,
				Kind:      controller.InputQPrimary,
			},
			{
				Namespace: resources.DefaultNamespace,
				Type:      siderolink.LinkType,
				Kind:      controller.InputQMapped,
			},
		},
		Outputs: []controller.Output{
			{
				Kind: controller.OutputShared,
				Type: siderolink.NodeUniqueTokenType,
			},
		},
		Concurrency: optional.Some[uint](4),
	}
}

// MapInput implements controller.QController interface.
func (ctrl *NodeUniqueTokenCleanupController) MapInput(ctx context.Context, _ *zap.Logger,
	r controller.QRuntime, ptr controller.ReducedResourceMetadata,
) ([]resource.Pointer, error) {
	if ptr.Type() == siderolink.LinkType {
		return []resource.Pointer{
			siderolink.NewNodeUniqueToken(ptr.ID()).Metadata(),
		}, nil
	}

	return nil, fmt.Errorf("unexpected resource type %q", ptr.Type())
}

// Reconcile implements controller.QController interface.
func (ctrl *NodeUniqueTokenCleanupController) Reconcile(ctx context.Context,
	logger *zap.Logger, r controller.QRuntime, ptr resource.Pointer,
) error {
	nodeUniqueToken, err := safe.ReaderGet[*siderolink.NodeUniqueToken](ctx, r, siderolink.NewNodeUniqueToken(ptr.ID()).Metadata())
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil
		}

		return err
	}

	if nodeUniqueToken.Metadata().Phase() == resource.PhaseTearingDown {
		return ctrl.reconcileTearingDown(ctx, r, nodeUniqueToken)
	}

	return ctrl.reconcileRunning(ctx, r, logger, nodeUniqueToken)
}

func (ctrl *NodeUniqueTokenCleanupController) reconcileRunning(ctx context.Context, r controller.QRuntime, logger *zap.Logger, nodeUniqueToken *siderolink.NodeUniqueToken) error {
	link, err := safe.ReaderGetByID[*siderolink.Link](ctx, r, nodeUniqueToken.Metadata().ID())
	if err != nil && !state.IsNotFoundError(err) {
		return err
	}

	if link == nil {
		if time.Since(nodeUniqueToken.Metadata().Created()) > ctrl.orphanTTL {
			_, err = helpers.TeardownAndDestroy(ctx, r, nodeUniqueToken.Metadata(), controller.WithOwner(nodeUniqueToken.Metadata().Owner()))

			logger.Info("remove orphaned node unique token")

			return err
		}

		return controller.NewRequeueInterval(ctrl.orphanTTL)
	}

	return nil
}

func (ctrl *NodeUniqueTokenCleanupController) reconcileTearingDown(ctx context.Context, r controller.QRuntime, nodeUniqueToken *siderolink.NodeUniqueToken) error {
	_, err := helpers.TeardownAndDestroy(ctx, r, nodeUniqueToken.Metadata(), controller.WithOwner(nodeUniqueToken.Metadata().Owner()))
	if err != nil {
		return err
	}

	return nil
}
