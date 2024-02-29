// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package cache implements a simple in-memory cache for a single value.
package cache

import (
	"sync"
	"time"
)

// Value is a cache for a single value.
//
//nolint:govet
type Value[T any] struct {
	// Duration is the duration for which the cached value is valid.
	Duration time.Duration

	mx         sync.Mutex
	lastResult T
	lastTime   time.Time
}

// GetOrUpdate returns the cached result if it is still valid, otherwise it calls the given function and caches the result.
func (c *Value[T]) GetOrUpdate(fn func() (T, error)) (T, error) {
	c.mx.Lock()
	defer c.mx.Unlock()

	if time.Since(c.lastTime) > c.Duration {
		var err error

		c.lastResult, err = fn()
		if err != nil {
			var zero T

			return zero, err
		}

		c.lastTime = time.Now()
	}

	return c.lastResult, nil
}
