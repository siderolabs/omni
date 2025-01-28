// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package validated

import (
	"context"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/hashicorp/go-multierror"
)

// State is a state that validates resources before passing them to the underlying state.
type State struct {
	st state.CoreState

	getValidations       []GetValidation
	listValidations      []ListValidation
	createValidations    []CreateValidation
	updateValidations    []UpdateValidation
	destroyValidations   []DestroyValidation
	watchValidations     []WatchValidation
	watchKindValidations []WatchKindValidation
}

// NewState creates a new validated state with the given underlying state and options.
func NewState(st state.CoreState, opts ...StateOption) state.CoreState {
	validatedState := &State{
		st: st,
	}

	for _, opt := range opts {
		opt(validatedState)
	}

	return validatedState
}

// Get gets a resource from the underlying state.
func (v *State) Get(ctx context.Context, pointer resource.Pointer, option ...state.GetOption) (resource.Resource, error) {
	res, err := v.st.Get(ctx, pointer, option...)
	if err != nil && !state.IsNotFoundError(err) {
		return nil, err
	}

	// if the existing resource was not found, instead of returning the not found error, run the validations first
	// only if the validations pass, return the not found error

	var validationErrs error

	for _, validation := range v.getValidations {
		if validationErr := validation(ctx, pointer, res, option...); validationErr != nil {
			validationErrs = multierror.Append(validationErrs, validationErr)
		}
	}

	if validationErrs != nil {
		return nil, ValidationError(validationErrs)
	}

	return res, err
}

// List lists resources from the underlying state.
func (v *State) List(ctx context.Context, kind resource.Kind, option ...state.ListOption) (resource.List, error) {
	var validationErrs error

	for _, validation := range v.listValidations {
		if validationErr := validation(ctx, kind, option...); validationErr != nil {
			validationErrs = multierror.Append(validationErrs, validationErr)
		}
	}

	if validationErrs != nil {
		return resource.List{}, ValidationError(validationErrs)
	}

	return v.st.List(ctx, kind, option...)
}

// Create creates a resource in the underlying state.
func (v *State) Create(ctx context.Context, resource resource.Resource, option ...state.CreateOption) error {
	var validationErrs error

	for _, validation := range v.createValidations {
		if validationErr := validation(ctx, resource, option...); validationErr != nil {
			validationErrs = multierror.Append(validationErrs, validationErr)
		}
	}

	if validationErrs != nil {
		return ValidationError(validationErrs)
	}

	return v.st.Create(ctx, resource, option...)
}

// Update updates a resource in the underlying state.
func (v *State) Update(ctx context.Context, newResource resource.Resource, opts ...state.UpdateOption) error {
	existing, err := v.st.Get(ctx, newResource.Metadata())
	if err != nil && !state.IsNotFoundError(err) {
		return err
	}

	var validationErrs error

	// if the existing resource was not found, instead of returning the not found error, run the validations first
	// only if the validations pass, return the not found error

	for _, validation := range v.updateValidations {
		if validationErr := validation(ctx, existing, newResource, opts...); validationErr != nil {
			validationErrs = multierror.Append(validationErrs, validationErr)
		}
	}

	// If the resource is tearing down, run the destroy validations as well
	if newResource.Metadata().Phase() == resource.PhaseTearingDown {
		updateOpts := state.UpdateOptions{}

		for _, opt := range opts {
			opt(&updateOpts)
		}

		for _, validation := range v.destroyValidations {
			if validationErr := validation(ctx, newResource.Metadata(), existing, state.WithDestroyOwner(updateOpts.Owner)); validationErr != nil {
				validationErrs = multierror.Append(validationErrs, validationErr)
			}
		}
	}

	if validationErrs != nil {
		return ValidationError(validationErrs)
	}

	if err != nil {
		return err
	}

	return v.st.Update(ctx, newResource, opts...)
}

// Destroy destroys a resource in the underlying state.
func (v *State) Destroy(ctx context.Context, pointer resource.Pointer, option ...state.DestroyOption) error {
	existing, err := v.st.Get(ctx, pointer)
	if err != nil && !state.IsNotFoundError(err) {
		return err
	}

	// if the existing resource was not found, instead of returning the not found error, run the validations first
	// only if the validations pass, return the not found error

	var validationErrs error

	for _, validation := range v.destroyValidations {
		if validationErr := validation(ctx, pointer, existing, option...); validationErr != nil {
			validationErrs = multierror.Append(validationErrs, validationErr)
		}
	}

	if validationErrs != nil {
		return ValidationError(validationErrs)
	}

	if err != nil {
		return err
	}

	return v.st.Destroy(ctx, pointer, option...)
}

// Watch watches a resource in the underlying state.
func (v *State) Watch(ctx context.Context, pointer resource.Pointer, events chan<- state.Event, option ...state.WatchOption) error {
	var validationErrs error

	for _, validation := range v.watchValidations {
		if validationErr := validation(ctx, pointer, option...); validationErr != nil {
			validationErrs = multierror.Append(validationErrs, validationErr)
		}
	}

	if validationErrs != nil {
		return ValidationError(validationErrs)
	}

	return v.st.Watch(ctx, pointer, events, option...)
}

// WatchKind watches a resource kind in the underlying state.
func (v *State) WatchKind(ctx context.Context, kind resource.Kind, events chan<- state.Event, option ...state.WatchKindOption) error {
	var validationErrs error

	for _, validation := range v.watchKindValidations {
		if validationErr := validation(ctx, kind, option...); validationErr != nil {
			validationErrs = multierror.Append(validationErrs, validationErr)
		}
	}

	if validationErrs != nil {
		return ValidationError(validationErrs)
	}

	return v.st.WatchKind(ctx, kind, events, option...)
}

// WatchKindAggregated watches a resource kind in the underlying state.
func (v *State) WatchKindAggregated(ctx context.Context, kind resource.Kind, c chan<- []state.Event, option ...state.WatchKindOption) error {
	var validationErrs error

	for _, validation := range v.watchKindValidations {
		if validationErr := validation(ctx, kind, option...); validationErr != nil {
			validationErrs = multierror.Append(validationErrs, validationErr)
		}
	}

	if validationErrs != nil {
		return ValidationError(validationErrs)
	}

	return v.st.WatchKindAggregated(ctx, kind, c, option...)
}
