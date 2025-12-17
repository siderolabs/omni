// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package migration COSI state migration management utilities.
package migration

import (
	"context"
	"fmt"
	"slices"

	"github.com/cosi-project/runtime/pkg/controller/generic"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
)

func dropFinalizers[R generic.ResourceWithRD](ctx context.Context, st state.State, finalizers ...resource.Finalizer) error {
	list, err := safe.StateListAll[R](ctx, st)
	if err != nil {
		return fmt.Errorf("failed to list resources: %w", err)
	}

	for res := range list.All() {
		hasAny := slices.ContainsFunc(finalizers, res.Metadata().Finalizers().Has)

		if !hasAny {
			continue
		}

		if err = st.RemoveFinalizer(ctx, res.Metadata(), finalizers...); err != nil {
			return fmt.Errorf("failed to remove finalizers from %s: %w", res.Metadata().ID(), err)
		}
	}

	return nil
}
