// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package cmd

import "github.com/spf13/cobra"

// BuildRootCommand is exposed for testing.
func BuildRootCommand() (*cobra.Command, error) {
	return buildRootCommand()
}
