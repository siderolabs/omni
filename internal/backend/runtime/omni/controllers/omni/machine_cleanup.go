// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"errors"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic"
	"github.com/cosi-project/runtime/pkg/controller/generic/cleanup"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xerrors"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// MachineCleanupController manages MachineCleanup resource lifecycle.
type MachineCleanupController = cleanup.Controller[*omni.Machine]

type sameIDHandler[I, O generic.ResourceWithRD] struct{}

func (h *sameIDHandler[I, O]) FinalizerRemoval(ctx context.Context, r controller.Runtime, _ *zap.Logger, input I) error {
	var zeroOut O

	md := resource.NewMetadata(
		zeroOut.ResourceDefinition().DefaultNamespace,
		zeroOut.ResourceDefinition().Type,
		input.Metadata().ID(),
		resource.VersionUndefined,
	)

	res, err := r.Get(ctx, md)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil
		}

		return err
	}

	if res.Metadata().Owner() != "" {
		return nil
	}

	ready, err := r.Teardown(ctx, md)
	if err != nil {
		return err
	}

	if !ready {
		return xerrors.NewTagged[cleanup.SkipReconcileTag](errors.New("waiting for resources to be destroyed"))
	}

	return r.Destroy(ctx, md, controller.WithOwner(""))
}

func (h *sameIDHandler[I, O]) Inputs() []controller.Input {
	return []controller.Input{
		{
			Type:      omni.MachineSetNodeType,
			Namespace: resources.DefaultNamespace,
			Kind:      controller.InputWeak,
		},
	}
}

func (h *sameIDHandler[I, O]) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: omni.MachineSetNodeType,
			Kind: controller.OutputShared,
		},
	}
}

// NewMachineCleanupController returns a new MachineCleanup controller.
// This controller should remove all MachineSetNodes for a tearing down machine.
// If the MachineSetNode is owned by some controller, it is skipped.
func NewMachineCleanupController() *MachineCleanupController {
	return cleanup.NewController(
		cleanup.Settings[*omni.Machine]{
			Name:    "MachineCleanupController",
			Handler: &sameIDHandler[*omni.Machine, *omni.MachineSetNode]{},
		},
	)
}
