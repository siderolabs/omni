// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package machine contains commands related to machine operations.
package machine

import (
	"github.com/siderolabs/gen/ensure"
	"github.com/spf13/cobra"
)

var machineCmdFlags struct {
	machine string
}

// machineCmd represents the cluster sub-command.
var machineCmd = &cobra.Command{
	Use:     "machine",
	Aliases: []string{"m"},
	Short:   "Machine-related subcommands.",
	Long:    `Commands to manage users.`,
	Example: "",
}

// RootCmd exposes root machine command.
func RootCmd() *cobra.Command {
	machineCmd.PersistentFlags().StringVar(&machineCmdFlags.machine, "id", "", "machine UUID")

	ensure.NoError(machineCmd.MarkPersistentFlagRequired("id"))

	return machineCmd
}
