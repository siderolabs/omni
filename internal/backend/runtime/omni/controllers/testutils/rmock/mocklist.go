// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package rmock

import (
	"context"
	"testing"

	"github.com/cosi-project/runtime/pkg/controller/generic"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/google/uuid"
	"github.com/siderolabs/gen/xslices"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils/rmock/options"
)

// MockList of resources.
func MockList[T generic.ResourceWithRD](ctx context.Context, t *testing.T, st state.State, optionList ...options.MockListOption) []T {
	var opts options.MockListOptions

	for _, opt := range optionList {
		opt(&opts)
	}

	if opts.IDQuery != nil {
		opts.IDs = opts.IDQuery(ctx, t, st)
	}

	if opts.IDs != nil && opts.Count != 0 {
		t.Fatalf("conflicting options IDs and count")
	}

	if opts.IDs != nil {
		return xslices.Map(opts.IDs, func(id string) T {
			mockOpts := append(opts.MockOptions, options.WithID(id)) //nolint:gocritic

			return Mock[T](ctx, t, st, mockOpts...)
		})
	}

	resources := make([]T, 0, opts.Count)

	for range opts.Count {
		mockOpts := append(opts.MockOptions, options.WithID(uuid.NewString())) //nolint:gocritic

		resources = append(resources, Mock[T](ctx, t, st, mockOpts...))
	}

	require.NotEmpty(t, resources, "no resources were created")

	return resources
}
