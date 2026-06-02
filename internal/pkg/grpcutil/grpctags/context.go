// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package grpctags provides request-scoped logging tags for gRPC interceptors.
//
// It is a trimmed-down version of the "tags" package from
// github.com/grpc-ecosystem/go-grpc-middleware v1, which was
// removed in v2.
package grpctags

import "context"

type ctxMarker struct{}

var (
	ctxMarkerKey      = &ctxMarker{}
	noTags       Tags = noopTags{}
)

// Tags stores request-scoped logging fields.
type Tags interface {
	Set(key string, value any) Tags
	Has(key string) bool
	Values() map[string]any
}

type mapTags struct {
	values map[string]any
}

func (t *mapTags) Set(key string, value any) Tags {
	t.values[key] = value

	return t
}

func (t *mapTags) Has(key string) bool {
	_, ok := t.values[key]

	return ok
}

func (t *mapTags) Values() map[string]any {
	return t.values
}

type noopTags struct{}

func (noopTags) Set(string, any) Tags {
	return noTags
}

func (noopTags) Has(string) bool {
	return false
}

func (noopTags) Values() map[string]any {
	return nil
}

// Extract returns request-scoped tags from ctx, or a no-op storage if none exists.
func Extract(ctx context.Context) Tags {
	t, ok := ctx.Value(ctxMarkerKey).(Tags)
	if !ok {
		return noTags
	}

	return t
}

// SetInContext stores tags in ctx.
func SetInContext(ctx context.Context, tags Tags) context.Context {
	return context.WithValue(ctx, ctxMarkerKey, tags)
}

// NewTags creates a mutable tag storage.
func NewTags() Tags {
	return &mapTags{values: make(map[string]any)}
}
