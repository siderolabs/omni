// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package user contains commands related to user operations.
package user

import (
	"github.com/spf13/cobra"
)

// userCmd represents the cluster sub-command.
var userCmd = &cobra.Command{
	Use:     "user",
	Aliases: []string{"u"},
	Short:   "User-related subcommands.",
	Long:    `Commands to manage users.`,
	Example: "",
}

// RootCmd exposes root cluster command.
func RootCmd() *cobra.Command {
	return userCmd
}
