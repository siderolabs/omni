// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package audit

import (
	"context"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
)

// WrapState wraps the given state with audit log state.
func WrapState(s state.State, l *Log) state.State {
	return &auditState{
		state:  s,
		logger: l,
	}
}

type auditState struct {
	state  state.State
	logger *Log
}

func (a *auditState) Create(ctx context.Context, res resource.Resource, option ...state.CreateOption) error {
	err := a.state.Create(ctx, res, option...)
	if err != nil {
		return err
	}

	if fn := a.logger.LogCreate(res); fn != nil {
		return fn(ctx, res, option...)
	}

	return nil
}

func (a *auditState) Update(ctx context.Context, newRes resource.Resource, opts ...state.UpdateOption) error {
	fn := a.logger.LogUpdate(newRes)
	if fn == nil {
		return a.state.Update(ctx, newRes, opts...)
	}

	oldRes, err := a.state.Get(ctx, newRes.Metadata())
	if err != nil && !state.IsNotFoundError(err) {
		return err
	}

	err = a.state.Update(ctx, newRes, opts...)
	if err != nil {
		return err
	}

	return fn(ctx, oldRes, newRes, opts...)
}

func (a *auditState) Destroy(ctx context.Context, ptr resource.Pointer, option ...state.DestroyOption) error {
	err := a.state.Destroy(ctx, ptr, option...)
	if err != nil {
		return err
	}

	if fn := a.logger.LogDestroy(ptr); fn != nil {
		return fn(ctx, ptr, option...)
	}

	return nil
}

func (a *auditState) UpdateWithConflicts(ctx context.Context, ptr resource.Pointer, updaterFunc state.UpdaterFunc, opts ...state.UpdateOption) (resource.Resource, error) {
	fn := a.logger.LogUpdateWithConflicts(ptr)
	if fn == nil {
		return a.state.UpdateWithConflicts(ctx, ptr, updaterFunc, opts...)
	}

	var oldRes resource.Resource

	newRes, err := a.state.UpdateWithConflicts(
		ctx,
		ptr,
		func(r resource.Resource) error {
			oldRes = r.DeepCopy()

			return updaterFunc(r)
		},
		opts...,
	)
	if err != nil {
		return nil, err
	}

	return newRes, fn(ctx, oldRes, newRes, opts...)
}

func (a *auditState) Get(ctx context.Context, ptr resource.Pointer, option ...state.GetOption) (resource.Resource, error) {
	return a.state.Get(ctx, ptr, option...)
}

func (a *auditState) List(ctx context.Context, kind resource.Kind, option ...state.ListOption) (resource.List, error) {
	return a.state.List(ctx, kind, option...)
}

func (a *auditState) Watch(ctx context.Context, ptr resource.Pointer, events chan<- state.Event, option ...state.WatchOption) error {
	return a.state.Watch(ctx, ptr, events, option...)
}

func (a *auditState) WatchKind(ctx context.Context, kind resource.Kind, events chan<- state.Event, option ...state.WatchKindOption) error {
	return a.state.WatchKind(ctx, kind, events, option...)
}

func (a *auditState) WatchKindAggregated(ctx context.Context, kind resource.Kind, c chan<- []state.Event, option ...state.WatchKindOption) error {
	return a.state.WatchKindAggregated(ctx, kind, c, option...)
}

func (a *auditState) WatchFor(ctx context.Context, pointer resource.Pointer, conditionFunc ...state.WatchForConditionFunc) (resource.Resource, error) {
	return a.state.WatchFor(ctx, pointer, conditionFunc...)
}

func (a *auditState) Teardown(ctx context.Context, pointer resource.Pointer, option ...state.TeardownOption) (bool, error) {
	return a.state.Teardown(ctx, pointer, option...)
}

func (a *auditState) AddFinalizer(ctx context.Context, pointer resource.Pointer, finalizer ...resource.Finalizer) error {
	return a.state.AddFinalizer(ctx, pointer, finalizer...)
}

func (a *auditState) RemoveFinalizer(ctx context.Context, pointer resource.Pointer, finalizer ...resource.Finalizer) error {
	return a.state.RemoveFinalizer(ctx, pointer, finalizer...)
}

func (a *auditState) ContextWithTeardown(ctx context.Context, pointer resource.Pointer) (context.Context, error) {
	return a.state.ContextWithTeardown(ctx, pointer)
}
