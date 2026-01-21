// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package rmock

import (
	"context"
	"fmt"
	"testing"

	"github.com/cosi-project/runtime/pkg/controller/generic"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/kvutils"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils/rmock/options"
)

// Mock a resource.
// It will pick the defaults, set the owner if it's defined.
func Mock[T generic.ResourceWithRD](ctx context.Context, t *testing.T, st state.State, optionsList ...options.MockOption) T {
	var opts options.MockOptions

	for _, opt := range optionsList {
		opt(&opts)
	}

	if opts.ID == "" {
		opts.ID = uuid.NewString()
	}

	var zero T

	r, err := protobuf.CreateResource(zero.ResourceDefinition().Type)

	require.NoError(t, err)

	res := r.(T) //nolint:forcetypeassert,errcheck

	*res.Metadata() = resource.NewMetadata(
		zero.ResourceDefinition().DefaultNamespace,
		zero.ResourceDefinition().Type,
		opts.ID,
		resource.VersionUndefined,
	)
	// initialize empty spec
	if r, ok := res.Spec().(interface {
		UnmarshalJSON(bytes []byte) error
	}); ok {
		require.NoError(t, r.UnmarshalJSON([]byte("{}")))
	}

	owner := owners[zero.ResourceDefinition().Type]

	result, err := safe.StateModifyWithResult(ctx, st, res, func(t T) error {
		t.Metadata().Labels().Do(func(temp kvutils.TempKV) {
			for key, value := range opts.Labels {
				temp.Set(key, value)
			}
		})

		defaultPreset, hasDefaultPreset := defaults[t.Metadata().Type()]
		if hasDefaultPreset {
			if err = defaultPreset(ctx, st, t); err != nil {
				if state.IsNotFoundError(err) {
					return fmt.Errorf("failed to mock %q, because %w", res.ResourceDefinition().Type, err)
				}

				return err
			}
		}

		for _, p := range opts.ModifyCallbacks {
			if err = p(t); err != nil {
				return err
			}
		}

		return nil
	}, state.WithUpdateOwner(owner))

	require.NoError(t, err)

	return result
}
