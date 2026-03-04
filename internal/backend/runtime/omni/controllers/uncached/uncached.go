// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package uncached provides wrappers around controller.Reader and controller.ReaderWriter
// that bypass the controller runtime's read cache by delegating all reads to GetUncached/ListUncached.
package uncached

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
)

type reader struct {
	inner controller.Reader
}

func (u *reader) Get(ctx context.Context, pointer resource.Pointer, option ...state.GetOption) (resource.Resource, error) {
	rdr, ok := u.inner.(controller.UncachedReader)
	if !ok {
		return nil, fmt.Errorf("reader does not support uncached reads")
	}

	return rdr.GetUncached(ctx, pointer, option...)
}

func (u *reader) List(ctx context.Context, kind resource.Kind, option ...state.ListOption) (resource.List, error) {
	rdr, ok := u.inner.(controller.UncachedReader)
	if !ok {
		return resource.List{}, fmt.Errorf("reader does not support uncached reads")
	}

	return rdr.ListUncached(ctx, kind, option...)
}

func (u *reader) ContextWithTeardown(ctx context.Context, pointer resource.Pointer) (context.Context, error) {
	return u.inner.ContextWithTeardown(ctx, pointer)
}

type readWriter struct {
	controller.Reader
	controller.Writer
}

// Reader wraps a controller.Reader so that all reads bypass the controller runtime cache.
func Reader(r controller.Reader) controller.Reader {
	return &reader{inner: r}
}

// ReaderWriter wraps a controller.ReaderWriter so that all reads bypass the controller runtime cache.
//
// Writes are passed through unchanged.
func ReaderWriter(r controller.ReaderWriter) controller.ReaderWriter {
	return &readWriter{
		Reader: &reader{inner: r},
		Writer: r,
	}
}
