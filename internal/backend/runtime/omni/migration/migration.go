// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package migration COSI state migration management utilities.
package migration

import (
	"context"

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
