// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package main initializes omnictl CLI.
package main

import (
	"os"

	"github.com/siderolabs/omni/client/pkg/omnictl"
	"github.com/siderolabs/omni/client/pkg/version"
	internalversion "github.com/siderolabs/omni/internal/version"
)

func main() {
	version.Name = internalversion.Name
	version.SHA = internalversion.SHA
	version.Tag = internalversion.Tag
	version.API = internalversion.API

	omnictl.RootCmd.Version = version.String()

	if err := omnictl.RootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
