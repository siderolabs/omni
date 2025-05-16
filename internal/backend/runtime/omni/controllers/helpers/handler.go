// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package helpers

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
)

// SameIDHandler is a cleanup handler that removes output resource that has the same ID as the input resource.
//
// It defines the input resource with the given InputKind, and skips the removal if the output resource is not owned by the given owner.
type SameIDHandler[I, O generic.ResourceWithRD] struct {
	Owner     string
	InputKind controller.InputKind
}

// FinalizerRemoval implements cleanup.Handler.
func (h *SameIDHandler[I, O]) FinalizerRemoval(ctx context.Context, r controller.Runtime, _ *zap.Logger, input I) error {
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

	if res.Metadata().Owner() != h.Owner {
		return nil
	}

	ready, err := r.Teardown(ctx, md, controller.WithOwner(h.Owner))
	if err != nil {
		return err
	}

	if !ready {
		return xerrors.NewTagged[cleanup.SkipReconcileTag](errors.New("waiting for resources to be destroyed"))
	}

	return r.Destroy(ctx, md, controller.WithOwner(h.Owner))
}

// Inputs implements cleanup.Handler.
func (h *SameIDHandler[I, O]) Inputs() []controller.Input {
	var zeroOut O

	return []controller.Input{{
		Namespace: zeroOut.ResourceDefinition().DefaultNamespace,
		Type:      zeroOut.ResourceDefinition().Type,
		Kind:      h.InputKind,
	}}
}

// Outputs implements cleanup.Handler.
func (h *SameIDHandler[I, O]) Outputs() []controller.Output {
	var zeroOut O

	return []controller.Output{
		{
			Type: zeroOut.ResourceDefinition().Type,
			Kind: controller.OutputShared,
		},
	}
}

// MapID finalizer removal.
func MapID[I, O generic.ResourceWithRD](idGetter func(I) resource.ID, blockIfOwnerDiffers bool) func(context.Context, controller.Runtime, I, string) error {
	return func(ctx context.Context, r controller.Runtime, input I, owner string) error {
		var zeroOut O

		md := resource.NewMetadata(
			zeroOut.ResourceDefinition().DefaultNamespace,
			zeroOut.ResourceDefinition().Type,
			idGetter(input),
			resource.VersionUndefined,
		)

		res, err := r.Get(ctx, md)
		if err != nil {
			if state.IsNotFoundError(err) {
				return nil
			}

			return err
		}

		if res.Metadata().Owner() != owner {
			if blockIfOwnerDiffers {
				return xerrors.NewTagged[cleanup.SkipReconcileTag](errors.New("waiting for resources to be destroyed"))
			}

			return nil
		}

		ready, err := r.Teardown(ctx, md, controller.WithOwner(owner))
		if err != nil {
			return err
		}

		if !ready {
			return xerrors.NewTagged[cleanup.SkipReconcileTag](errors.New("waiting for resources to be destroyed"))
		}

		return r.Destroy(ctx, md, controller.WithOwner(owner))
	}
}

// CustomHandler is a cleanup handler that removes output resource that has the ID extracted from the resource by the custom function.
//
// It defines the input resource with the given InputKind, and skips the removal if the output resource is not owned by the given owner.
type CustomHandler[I, O generic.ResourceWithRD] struct {
	handler      func(ctx context.Context, r controller.Runtime, input I, owner string) error
	Owner        string
	extraOutputs []controller.Output
	InputKind    controller.InputKind
	noOutputs    bool
}

// NewCustomHandler creates a new custom cleanup handler.
func NewCustomHandler[I, O generic.ResourceWithRD](handler func(ctx context.Context, r controller.Runtime, input I, owner string) error, noOutputs bool,
	extraOutputs ...controller.Output,
) *CustomHandler[I, O] {
	return &CustomHandler[I, O]{handler: handler, noOutputs: noOutputs, extraOutputs: extraOutputs}
}

// FinalizerRemoval implements cleanup.Handler.
func (h *CustomHandler[I, O]) FinalizerRemoval(ctx context.Context, r controller.Runtime, _ *zap.Logger, input I) error {
	return h.handler(ctx, r, input, h.Owner)
}

// Inputs implements cleanup.Handler.
func (h *CustomHandler[I, O]) Inputs() []controller.Input {
	var zeroOut O

	return []controller.Input{{
		Namespace: zeroOut.ResourceDefinition().DefaultNamespace,
		Type:      zeroOut.ResourceDefinition().Type,
		Kind:      h.InputKind,
	}}
}

// Outputs implements cleanup.Handler.
func (h *CustomHandler[I, O]) Outputs() []controller.Output {
	if h.noOutputs {
		return nil
	}

	var zeroOut O

	return append(h.extraOutputs,
		controller.Output{
			Type: zeroOut.ResourceDefinition().Type,
			Kind: controller.OutputShared,
		},
	)
}
