// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package main initializes omnictl CLI.
package main

import (
	"errors"
	"os"

	"github.com/siderolabs/omni/client/pkg/execdiff"
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
		// Differences-found is a signaling error (dry-run showed a diff),
		// not a runtime failure. Map it to exit 1 silently so the documented
		// contract holds: 0 = no diff, 1 = diff found, >1 = error.
		// Cobra prints all other errors itself (including flag-parse errors),
		// so main does not print again to avoid duplicate "Error:" lines.
		if errors.Is(err, execdiff.ErrDifferencesFound) {
			os.Exit(1)
		}

		os.Exit(2)
	}
}
