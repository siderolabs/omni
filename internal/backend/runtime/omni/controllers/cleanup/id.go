// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package cleanup

import (
	"context"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic"
	"github.com/cosi-project/runtime/pkg/controller/generic/cleanup"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xerrors"
	"go.uber.org/zap"
)

// SameIDHandler is a cleanup handler that removes the output resource that has the same ID as the input resource.
//
// It skips the removal if the given owner does not own the output resource.
type SameIDHandler[I, O generic.ResourceWithRD] struct{}

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

	// Here, we do a cleanup only for the user-owned resources, i.e., the resources with an empty owner.
	//
	// The resources that are owned by controllers should be destroyed by the respective controllers.
	expectedOwner := ""

	if res.Metadata().Owner() != expectedOwner {
		return nil
	}

	ready, err := r.Teardown(ctx, md, controller.WithOwner(expectedOwner))
	if err != nil {
		return err
	}

	if !ready {
		return xerrors.NewTaggedf[cleanup.SkipReconcileTag]("waiting for resources to be destroyed")
	}

	return r.Destroy(ctx, md, controller.WithOwner(expectedOwner))
}

// Inputs implements cleanup.Handler.
func (h *SameIDHandler[I, O]) Inputs() []controller.Input {
	var zeroOut O

	return []controller.Input{{
		Namespace: zeroOut.ResourceDefinition().DefaultNamespace,
		Type:      zeroOut.ResourceDefinition().Type,
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

type GetIDFunc[I generic.ResourceWithRD] func(I) resource.ID

// IDHandleFunc maps the input resource to the output resource, extracting the ID from the input resource using the given GetIDFunc.
func IDHandleFunc[I, O generic.ResourceWithRD](getIDFunc GetIDFunc[I], blockIfOwnerDiffers bool) HandlerFunc[I] {
	return func(ctx context.Context, r controller.Runtime, input I) error {
		var zeroOut O

		md := resource.NewMetadata(
			zeroOut.ResourceDefinition().DefaultNamespace,
			zeroOut.ResourceDefinition().Type,
			getIDFunc(input),
			resource.VersionUndefined,
		)

		res, err := r.Get(ctx, md)
		if err != nil {
			if state.IsNotFoundError(err) {
				return nil
			}

			return err
		}

		// Here, we check if the owner is empty, i.e., this is a user-owned resource.
		expectedOwner := ""

		if res.Metadata().Owner() != expectedOwner {
			if blockIfOwnerDiffers { // wait until the controller-owned resource is destroyed by its respective controller
				return xerrors.NewTaggedf[cleanup.SkipReconcileTag]("waiting for resources to be destroyed")
			}

			return nil // skip the controller-owned resource
		}

		ready, err := r.Teardown(ctx, md, controller.WithOwner(res.Metadata().Owner()))
		if err != nil {
			return err
		}

		if !ready {
			return xerrors.NewTaggedf[cleanup.SkipReconcileTag]("waiting for resources to be destroyed")
		}

		return r.Destroy(ctx, md, controller.WithOwner(expectedOwner))
	}
}
