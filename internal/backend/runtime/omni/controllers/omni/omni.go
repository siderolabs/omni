// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package omni contains controllers that are managing Omni resources: Machines, Clusters, etc..
package omni

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic"
	"github.com/cosi-project/runtime/pkg/controller/generic/cleanup"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"
)

type cleanupOptions struct {
	destroyReadyCallback func() error
}

type cleanupOpt func(*cleanupOptions)

func withDestroyReadyCallback(cb func() error) cleanupOpt {
	return func(opts *cleanupOptions) {
		opts.destroyReadyCallback = cb
	}
}

func trackResource(r controller.ReaderWriter, ns resource.Namespace, resourceType resource.Type, listOptions ...state.ListOption) *resourceTracker {
	return &resourceTracker{
		touched:     map[string]struct{}{},
		listMD:      resource.NewMetadata(ns, resourceType, "", resource.VersionUndefined),
		listOptions: listOptions,
		r:           r,
	}
}

type resourceTracker struct {
	owner                 string
	r                     controller.ReaderWriter
	beforeDestroyCallback func(resource.Resource) error
	touched               map[resource.ID]struct{}
	listOptions           []state.ListOption
	listMD                resource.Metadata
}

func (rt *resourceTracker) keep(res resource.Resource) {
	rt.touched[res.Metadata().ID()] = struct{}{}
}

func (rt *resourceTracker) getItemsToDestroy(ctx context.Context) ([]resource.Resource, error) {
	var resources []resource.Resource

	list, err := rt.r.List(ctx, rt.listMD, rt.listOptions...)
	if err != nil {
		return nil, fmt.Errorf("error listing resources: %w", err)
	}

	for _, res := range list.Items {
		if _, ok := rt.touched[res.Metadata().ID()]; !ok {
			resources = append(resources, res)
		}
	}

	return resources, nil
}

func (rt *resourceTracker) cleanup(ctx context.Context, cleanupOpts ...cleanupOpt) error {
	opts := &cleanupOptions{}
	for _, o := range cleanupOpts {
		o(opts)
	}

	items, err := rt.getItemsToDestroy(ctx)
	if err != nil {
		return fmt.Errorf("error getting items to destroy: %w", err)
	}

	destroyReady := true

	for _, res := range items {
		if rt.owner != "" && res.Metadata().Owner() != rt.owner {
			continue
		}

		var ready bool

		if ready, err = rt.r.Teardown(ctx, res.Metadata()); err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error tearing down resource '%s': %w", res.Metadata().ID(), err)
		}

		if !ready {
			destroyReady = false

			continue
		}

		if rt.beforeDestroyCallback != nil {
			if err = rt.beforeDestroyCallback(res); err != nil {
				return fmt.Errorf("error running before destroy callback for resource '%s': %w", res.Metadata().ID(), err)
			}
		}

		if err = rt.r.Destroy(ctx, res.Metadata()); err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error destroying resource '%s': %w", res.Metadata().ID(), err)
		}
	}

	if destroyReady && opts.destroyReadyCallback != nil {
		return opts.destroyReadyCallback()
	}

	return nil
}

// BeforeDestroy register the callback which is called before destroying the resource.
func (rt *resourceTracker) BeforeDestroy(f func(res resource.Resource) error) {
	rt.beforeDestroyCallback = f
}

// withFinalizerCheck wraps a [cleanup.Handler] with a check that needs to pass before the handler is called.
func withFinalizerCheck[Input generic.ResourceWithRD](handler cleanup.Handler[Input], check func(input Input) error) cleanup.Handler[Input] {
	return &cleanupChecker[Input]{
		next:  handler,
		check: check,
	}
}

type cleanupChecker[Input generic.ResourceWithRD] struct {
	next  cleanup.Handler[Input]
	check func(input Input) error
}

// FinalizerRemoval implements [cleanup.Handler].
// It first runs the check, and only if it passes, it calls the underlying handler's FinalizerRemoval.
func (c cleanupChecker[Input]) FinalizerRemoval(ctx context.Context, r controller.Runtime, logger *zap.Logger, input Input) error {
	if err := c.check(input); err != nil {
		logger.Warn("failed to cleanup outputs for resource", zap.Error(err), zap.String("resource_id", input.Metadata().ID()))

		return nil
	}

	if err := c.next.FinalizerRemoval(ctx, r, logger, input); err != nil {
		return err
	}

	return nil
}

// Inputs implements [cleanup.Handler].
func (c cleanupChecker[Input]) Inputs() []controller.Input {
	return c.next.Inputs()
}

// Outputs implements [cleanup.Handler].
func (c cleanupChecker[Input]) Outputs() []controller.Output {
	return c.next.Outputs()
}
