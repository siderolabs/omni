// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package main provides the entrypoint for the omni binary.
package main

import (
	"os"

	_ "github.com/siderolabs/omni/cmd/acompat" // this package should always be imported first for init->set env to work
	"github.com/siderolabs/omni/cmd/omni/cmd"
)

func main() {
	if err := cmd.RootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
