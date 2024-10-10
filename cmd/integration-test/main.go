// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package main provides the entrypoint for the omni-integration-test binary.
package main

import (
	"os"

	_ "github.com/siderolabs/omni/cmd/acompat" // this package should always be imported first for init->set env to work
	"github.com/siderolabs/omni/cmd/integration-test/pkg"
)

func main() {
	if err := pkg.RootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
