// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package options

import (
	"github.com/cosi-project/runtime/pkg/controller/generic"
	"github.com/cosi-project/runtime/pkg/resource"
)

// SameID uses same ID as the resource.
func SameID(res resource.Resource) MockOption {
	return WithID(res.Metadata().ID())
}

// WithID creates the mock with id.
func WithID(id string) MockOption {
	return func(o *MockOptions) {
		o.ID = id
	}
}

type MockOption func(*MockOptions)

// ResourceModify is used to modify the mocked resource.
type ResourceModify[T generic.ResourceWithRD] func(res T) error

// Modify the resource while creating/updating.
func Modify[T generic.ResourceWithRD](callback ResourceModify[T]) MockOption {
	return func(mo *MockOptions) {
		mo.ModifyCallbacks = append(mo.ModifyCallbacks, func(res resource.Resource) error {
			return callback(res.(T)) //nolint:forcetypeassert,errcheck
		})
	}
}

// MockOptions defines the options for a single resource mock.
type MockOptions struct {
	ID              string
	Labels          map[string]string
	ModifyCallbacks []func(res resource.Resource) error
}

// AddLabel to the mock options.
func (mo *MockOptions) AddLabel(key, value string) {
	if mo.Labels == nil {
		mo.Labels = map[string]string{}
	}

	mo.Labels[key] = value
}
