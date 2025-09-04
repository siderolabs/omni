// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package migration COSI state migration management utilities.
package migration

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller/generic"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
)

func createOrUpdate[T resource.Resource](ctx context.Context, s state.State, res T, update func(T) error, owner string, updateOpts ...state.UpdateOption) error {
	if err := update(res); err != nil {
		return err
	}

	for key, value := range res.Metadata().Labels().Raw() {
		res.Metadata().Labels().Set(key, value)
	}

	var createOpts []state.CreateOption

	if owner != "" {
		createOpts = append(createOpts, state.WithCreateOwner(owner))
		updateOpts = append(updateOpts, state.WithUpdateOwner(owner))
	}

	err := s.Create(ctx, res, createOpts...)
	if err == nil {
		return nil
	}

	if !state.IsConflictError(err) {
		return err
	}

	if _, err := safe.StateUpdateWithConflicts(ctx, s, res.Metadata(), update, updateOpts...); err != nil {
		// ignore phase conflicts
		if state.IsPhaseConflictError(err) {
			return nil
		}

		return err
	}

	return nil
}

func dropFinalizers[R generic.ResourceWithRD](ctx context.Context, st state.State, finalizers ...resource.Finalizer) error {
	list, err := safe.StateListAll[R](ctx, st)
	if err != nil {
		return fmt.Errorf("failed to list resources: %w", err)
	}

	for res := range list.All() {
		hasAny := false

		for _, finalizer := range finalizers {
			if res.Metadata().Finalizers().Has(finalizer) {
				hasAny = true

				break
			}
		}

		if !hasAny {
			continue
		}

		if err = st.RemoveFinalizer(ctx, res.Metadata(), finalizers...); err != nil {
			return fmt.Errorf("failed to remove finalizers from %s: %w", res.Metadata().ID(), err)
		}
	}

	return nil
}
