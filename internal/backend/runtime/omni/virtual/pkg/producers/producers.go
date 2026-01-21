// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package producers contains different resource kinds watch implementations.
package producers

// Producer computes virtual resource and puts it to the inmem cache.
type Producer interface {
	Start() error
	Stop()
	Cleanup()
}
