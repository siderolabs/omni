// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package cleanup contains custom cleanup handlers to be used with cleanup.Controller.
package cleanup

import (
	"context"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic"
	"go.uber.org/zap"
)

type HandlerOptions struct {
	ExtraOutputs []controller.Output
	InputKind    controller.InputKind
	// NoOutputs skips registering the output of this handler as an output to the controller.
	//
	// This changes the behavior of this handler, where it doesn't do cleanup anymore, but only does some pre-checks / waits for conditions to be met.
	NoOutputs bool
}

// HandlerFunc is a custom cleanup handler function.
type HandlerFunc[I generic.ResourceWithRD] func(ctx context.Context, r controller.Runtime, input I) error

// Handler is a custom cleanup handler. It implements the cleanup.Handler interface.
//
// TODO: Refactor the cleanup.Controller to be a QController. This would simplify the cleanup logic quite a bit.
type Handler[I, O generic.ResourceWithRD] struct {
	handleFunc HandlerFunc[I]
	options    HandlerOptions
}

// FinalizerRemoval implements cleanup.Handler.
func (h *Handler[I, O]) FinalizerRemoval(ctx context.Context, r controller.Runtime, _ *zap.Logger, input I) error {
	return h.handleFunc(ctx, r, input)
}

// Inputs implements cleanup.Handler.
//
// The cleanup.Controller which uses this handler adds the default input (I) already as a controller.InputStrong, therefore, this method
// only needs to add the additional inputs we are interested in.
func (h *Handler[I, O]) Inputs() []controller.Input {
	// Here, we add the output of this controller as an additional input to the controller.
	// This makes sense in this specific case, since we are interested in the changes happening in our output.
	// We do not add it with the type controller.InputDestroyReady, because we want to wake up on all destroy events, even if they happen outside this controller.
	var zeroOut O

	return []controller.Input{{
		Namespace: zeroOut.ResourceDefinition().DefaultNamespace,
		Type:      zeroOut.ResourceDefinition().Type,
		Kind:      h.options.InputKind,
	}}
}

// Outputs implements cleanup.Handler.
//
// It defines the outputs of this handler as a controller.OutputShared controller output, unless the HandlerOptions.NoOutputs option is set.
func (h *Handler[I, O]) Outputs() []controller.Output {
	if h.options.NoOutputs {
		return nil
	}

	var zeroOut O

	return append(h.options.ExtraOutputs, []controller.Output{
		{
			Type: zeroOut.ResourceDefinition().Type,
			Kind: controller.OutputShared,
		},
	}...)
}

func NewHandler[I, O generic.ResourceWithRD](handleFunc HandlerFunc[I], options HandlerOptions) *Handler[I, O] {
	return &Handler[I, O]{
		handleFunc: handleFunc,
		options:    options,
	}
}
