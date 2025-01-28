// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package xcontext provides a small utils for context package
package xcontext

import "context"

// AfterFuncSync is like [context.AfterFunc] but it blocks until the function is executed.
func AfterFuncSync(ctx context.Context, fn func()) func() bool {
	stopChan := make(chan struct{})

	stop := context.AfterFunc(ctx, func() {
		defer close(stopChan)

		fn()
	})

	return func() bool {
		result := stop()
		if !result {
			<-stopChan
		}

		return result
	}
}
