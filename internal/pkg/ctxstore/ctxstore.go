// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package ctxstore provides a way to store values in the context with the key based on the type.
package ctxstore

import "context"

// phantomKey represent key based on type. The cool thing about this empty struct,
// is that two instances of phantomKey with the different type are different, while
// two instances of phantomKey with the same type are the same. This is useful for
// creating a unique key for each type. It also helps to avoid collision with other
// keys in the context.
//
// It also does not allocate when used as a key in the context (aka converted to any).
// Same goes for int and struct containing single int field with value below 256.
// Same goes for bool and struct containing single bool field.
type phantomKey[T any] struct{}

// WithValue creates a new context with the value. Key is based on the type of the value.
func WithValue[T any](ctx context.Context, val T) context.Context {
	return context.WithValue(ctx, phantomKey[T]{}, val)
}

// Value returns the value from the context. Key is based on the type of the value.
func Value[T any](ctx context.Context) (T, bool) {
	value := ctx.Value(phantomKey[T]{})
	if value == nil {
		return *new(T), false
	}

	return value.(T), true //nolint:forcetypeassert
}
