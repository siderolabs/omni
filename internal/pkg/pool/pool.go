// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package pool provides a generic pool for a specific type.
package pool

import "sync"

// Pool is a generic pool for a specific type.
// The New field is a function that creates a new instance of the type.
// It should be set before calling Get.
//
//nolint:govet
type Pool[T any] struct {
	New func() *T

	once sync.Once
	pool sync.Pool
}

// Get returns a new instance of the type from the pool.
func (p *Pool[T]) Get() *T {
	p.once.Do(func() { p.pool.New = func() any { return p.New() } })

	return p.pool.Get().(*T) //nolint:forcetypeassert
}

// Put returns an instance of the type to the pool.
func (p *Pool[T]) Put(x *T) {
	p.pool.Put(x)
}
