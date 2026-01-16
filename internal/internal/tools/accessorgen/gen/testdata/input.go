// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build tools

package testdata

import "time"

type Simple struct {
	Name *string
	Age  *int
}

type Complex struct {
	Tags    *[]string
	Timeout *time.Duration
	// Should not generate for these
	Normal string
	Slice  []int
}
