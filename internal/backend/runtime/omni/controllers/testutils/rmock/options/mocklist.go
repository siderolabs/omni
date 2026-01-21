// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package options

import (
	"context"
	"testing"

	"github.com/cosi-project/runtime/pkg/controller/generic"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/state"
)

type MockListOption func(*MockListOptions)

func ItemOptions(opts ...MockOption) MockListOption {
	return func(mn *MockListOptions) {
		mn.MockOptions = opts
	}
}

func Count(c int) MockListOption {
	return func(o *MockListOptions) {
		o.Count = c
	}
}

func QueryIDs[T generic.ResourceWithRD](queries ...resource.LabelQueryOption) MockListOption {
	return func(o *MockListOptions) {
		o.IDQuery = func(ctx context.Context, t *testing.T, st state.State) []string {
			return rtestutils.ResourceIDs[T](ctx, t, st, state.WithLabelQuery(queries...))
		}
	}
}

func IDs(ids []string) MockListOption {
	return func(o *MockListOptions) {
		o.IDs = ids
	}
}

type MockListOptions struct {
	IDQuery     func(context.Context, *testing.T, state.State) []string
	IDs         []string
	MockOptions []MockOption
	Count       int
}
