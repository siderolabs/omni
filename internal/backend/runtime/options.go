// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package runtime

import "strings"

// QueryOptions List and Get query options.
type QueryOptions struct {
	Namespace      string
	Name           string
	Context        string
	Resource       string
	FieldSelector  string
	CurrentVersion string
	SortField      string
	SearchFor      []string
	Nodes          []string
	LabelSelectors []string
	SortDescending bool
	TeardownOnly   bool
	TailEvents     int
	Offset         int
	Limit          int
}

// NewQueryOptions creates new QueryOptions.
func NewQueryOptions(setters ...QueryOption) *QueryOptions {
	o := &QueryOptions{}

	for _, s := range setters {
		s(o)
	}

	return o
}

// QueryOption defines variable input option for List and Get methods in the Runtime implementations.
type QueryOption func(*QueryOptions)

// WithNamespace enables filtering by namespace.
func WithNamespace(namespace string) QueryOption {
	return func(o *QueryOptions) {
		o.Namespace = namespace
	}
}

// WithName enables filtering by resource name.
func WithName(name string) QueryOption {
	return func(o *QueryOptions) {
		o.Name = name
	}
}

// WithLabelSelectors enables filtering by label.
func WithLabelSelectors(selectors ...string) QueryOption {
	return func(o *QueryOptions) {
		var labelSelectors []string

		for _, s := range selectors {
			labelSelectors = append(labelSelectors, strings.Split(s, ";")...)
		}

		o.LabelSelectors = labelSelectors
	}
}

// WithFieldSelector enables filtering by field.
func WithFieldSelector(selector string) QueryOption {
	return func(o *QueryOptions) {
		o.FieldSelector = selector
	}
}

// WithContext routes the request to a specific context.
func WithContext(name string) QueryOption {
	return func(o *QueryOptions) {
		o.Context = name
	}
}

// WithResource explicitly specifies the resource type to get from the runtime.
func WithResource(resource string) QueryOption {
	return func(o *QueryOptions) {
		o.Resource = resource
	}
}

// WithNodes explicitly defines nodes list to use for the request (Talos only).
func WithNodes(nodes ...string) QueryOption {
	return func(o *QueryOptions) {
		o.Nodes = nodes
	}
}

// WithCurrentVersion pass current version to the update call to avoid conflicts (only for update, Omni only).
func WithCurrentVersion(version string) QueryOption {
	return func(o *QueryOptions) {
		o.CurrentVersion = version
	}
}

// WithTailEvents requests resource backlog instead of the latest resource state.
func WithTailEvents(count int) QueryOption {
	return func(o *QueryOptions) {
		o.TailEvents = count
	}
}

// WithLimit limits the number of returned items.
func WithLimit(limit int) QueryOption {
	return func(o *QueryOptions) {
		o.Limit = limit
	}
}

// WithOffset skips the number of returned items.
func WithOffset(offset int) QueryOption {
	return func(o *QueryOptions) {
		o.Offset = offset
	}
}

// WithSort enables sorting by field.
func WithSort(field string, descending bool) QueryOption {
	return func(o *QueryOptions) {
		o.SortField = field
		o.SortDescending = descending
	}
}

// WithSearchFor enables field search.
func WithSearchFor(searchFor []string) QueryOption {
	return func(o *QueryOptions) {
		o.SearchFor = searchFor
	}
}

// WithTeardownOnly makes delete run teardown only (COSI only).
func WithTeardownOnly() QueryOption {
	return func(o *QueryOptions) {
		o.TeardownOnly = true
	}
}
