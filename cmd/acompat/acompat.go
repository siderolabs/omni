// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package acompat ensures that we have GRPC_ENFORCE_ALPN_ENABLED set to false.
// **Important**: This package should be lexically first in the main package.
package acompat

import "os"

func init() {
	if err := os.Setenv("GRPC_ENFORCE_ALPN_ENABLED", "false"); err != nil {
		panic(err)
	}
}
