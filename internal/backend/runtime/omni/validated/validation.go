// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package validated

import (
	"context"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/omni/client/pkg/constants"
)

// CreateValidation is a function that can be used to validate a resource before it is created.
type CreateValidation func(ctx context.Context, res resource.Resource, option ...state.CreateOption) error

// TypedCreateValidation is a function that can be used to validate a specific type of resource before it is created.
type TypedCreateValidation[T resource.Resource] func(ctx context.Context, res T, option ...state.CreateOption) error

// NewCreateValidationForType creates a CreateValidation for a specific type of resource.
// For all other resource types, the validation will be a no-op.
func NewCreateValidationForType[T resource.Resource](validation TypedCreateValidation[T]) CreateValidation {
	return func(ctx context.Context, res resource.Resource, option ...state.CreateOption) error {
		if constants.IsDebugBuild {
			if _, ok := res.Metadata().Annotations().Get(constants.DisableValidation); ok {
				return nil
			}
		}

		typedRes, ok := res.(T)
		if !ok {
			return nil
		}

		return validation(ctx, typedRes, option...)
	}
}

// UpdateValidation is a function that can be used to validate a resource before it is updated.
//
// NOTE: existingRes can be nil if the resource is not found.
// Instead of NotFound error being returned directly, validations will still run,
// and if they return an error, that error will be returned instead of NotFound error.
type UpdateValidation func(ctx context.Context, existingRes resource.Resource, newRes resource.Resource, option ...state.UpdateOption) error

// TypedUpdateValidation is a function that can be used to validate a specific type of resource before it is updated.
//
// NOTE: existingRes can be nil if the resource is not found.
// Instead of NotFound error being returned directly, validations will still run,
// and if they return an error, that error will be returned instead of NotFound error.
type TypedUpdateValidation[T resource.Resource] func(ctx context.Context, existingRes, newRes T, option ...state.UpdateOption) error

// NewUpdateValidationForType creates an UpdateValidation for a specific type of resource.
// For all other resource types, the validation will be a no-op.
func NewUpdateValidationForType[T resource.Resource](validation TypedUpdateValidation[T]) UpdateValidation {
	return func(ctx context.Context, existingRes resource.Resource, newRes resource.Resource, option ...state.UpdateOption) error {
		if constants.IsDebugBuild {
			if _, ok := newRes.Metadata().Annotations().Get(constants.DisableValidation); ok {
				return nil
			}
		}

		existingResTyped, ok := existingRes.(T)
		if !ok {
			return nil
		}

		newResTyped, ok := newRes.(T)
		if !ok {
			return nil
		}

		return validation(ctx, existingResTyped, newResTyped, option...)
	}
}

// DestroyValidation is a function that can be used to validate a resource before it is destroyed.
//
// NOTE: existingRes can be nil if the resource is not found.
// Instead of NotFound error being returned directly, validations will still run,
// and if they return an error, that error will be returned instead of NotFound error.
type DestroyValidation func(ctx context.Context, ptr resource.Pointer, existingRes resource.Resource, option ...state.DestroyOption) error

// TypedDestroyValidation is a function that can be used to validate a specific type of resource before it is destroyed.
//
// NOTE: existingRes can be nil if the resource is not found.
// Instead of NotFound error being returned directly, validations will still run,
// and if they return an error, that error will be returned instead of NotFound error.
type TypedDestroyValidation[T resource.Resource] func(ctx context.Context, ptr resource.Pointer, existingRes T, option ...state.DestroyOption) error

// NewDestroyValidationForType creates a DestroyValidation for a specific type of resource.
func NewDestroyValidationForType[T resource.Resource](validation TypedDestroyValidation[T]) DestroyValidation {
	return func(ctx context.Context, ptr resource.Pointer, res resource.Resource, option ...state.DestroyOption) error {
		if constants.IsDebugBuild && res != nil {
			if _, ok := res.Metadata().Annotations().Get(constants.DisableValidation); ok {
				return nil
			}
		}

		typedRes, ok := res.(T)
		if !ok {
			return nil
		}

		return validation(ctx, ptr, typedRes, option...)
	}
}

// GetValidation is a function that can be used to validate a resource after it is retrieved.
//
// NOTE: res can be nil if the resource is not found.
// Instead of NotFound error being returned directly, validations will still run,
// and if they return an error, that error will be returned instead of NotFound error.
type GetValidation func(ctx context.Context, ptr resource.Pointer, res resource.Resource, option ...state.GetOption) error

// ListValidation is a function that can be used to validate a resource before it is listed.
type ListValidation func(ctx context.Context, kind resource.Kind, option ...state.ListOption) error

// WatchValidation is a function that can be used to validate a resource before it is watched.
type WatchValidation func(ctx context.Context, ptr resource.Pointer, option ...state.WatchOption) error

// WatchKindValidation is a function that can be used to validate a resource before it is watched.
type WatchKindValidation func(ctx context.Context, kind resource.Kind, option ...state.WatchKindOption) error
