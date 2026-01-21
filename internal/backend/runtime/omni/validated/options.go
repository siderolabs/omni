// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package validated

// StateOption is a function that can be used to configure a state.
type StateOption func(*State)

// WithGetValidations adds validations to the state that are executed when a resource is retrieved.
func WithGetValidations(validations ...GetValidation) StateOption {
	return func(s *State) {
		s.getValidations = append(s.getValidations, validations...)
	}
}

// WithListValidations adds validations to the state that are executed when a resource is listed.
func WithListValidations(validations ...ListValidation) StateOption {
	return func(s *State) {
		s.listValidations = append(s.listValidations, validations...)
	}
}

// WithCreateValidations adds validations to the state that are executed when a resource is created.
func WithCreateValidations(validations ...CreateValidation) StateOption {
	return func(s *State) {
		s.createValidations = append(s.createValidations, validations...)
	}
}

// WithUpdateValidations adds validations to the state that are executed when a resource is updated.
func WithUpdateValidations(validations ...UpdateValidation) StateOption {
	return func(s *State) {
		s.updateValidations = append(s.updateValidations, validations...)
	}
}

// WithDestroyValidations adds validations to the state that are executed when a resource is destroyed.
func WithDestroyValidations(validations ...DestroyValidation) StateOption {
	return func(s *State) {
		s.destroyValidations = append(s.destroyValidations, validations...)
	}
}

// WithWatchValidations adds validations to the state that are executed when a resource is watched.
func WithWatchValidations(validations ...WatchValidation) StateOption {
	return func(s *State) {
		s.watchValidations = append(s.watchValidations, validations...)
	}
}

// WithWatchKindValidations adds validations to the state that are executed when a resource is watched.
func WithWatchKindValidations(validations ...WatchKindValidation) StateOption {
	return func(s *State) {
		s.watchKindValidations = append(s.watchKindValidations, validations...)
	}
}
