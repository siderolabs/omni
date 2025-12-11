// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package secret contains the commands related to cluster secrets.
package secret

import (
	"github.com/siderolabs/gen/ensure"
	"github.com/spf13/cobra"
)

var secretCmdFlags struct {
	clusterName string
}

// secretCmd represents the template sub-command.
var secretCmd = &cobra.Command{
	Use:     "secret",
	Short:   "Cluster secret management subcommands.",
	Long:    `Commands to manage cluster secrets.`,
	Example: "",
}

// RootCmd exposes root secret command.
func RootCmd() *cobra.Command {
	secretCmd.PersistentFlags().StringVarP(&secretCmdFlags.clusterName, "name", "n", "", "cluster name")

	ensure.NoError(secretCmd.MarkPersistentFlagRequired("name"))

	return secretCmd
}
