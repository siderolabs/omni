// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package template contains commands related to cluster template operations.
package template

import (
	"github.com/siderolabs/gen/ensure"
	"github.com/spf13/cobra"
)

// cmdFlags contains shared cluster template flags.
var cmdFlags struct {
	// Path to the cluster template file.
	TemplatePath string
}

// templateCmd represents the template sub-command.
var templateCmd = &cobra.Command{
	Use:     "template",
	Aliases: []string{"t"},
	Short:   "Cluster template management subcommands.",
	Long:    `Commands to render, validate, manage cluster templates.`,
	Example: "",
}

// RootCmd exports templateCmd.
func RootCmd() *cobra.Command {
	return templateCmd
}

func addRequiredFileFlag(cmd *cobra.Command) {
	cmd.PersistentFlags().StringVarP(&cmdFlags.TemplatePath, "file", "f", "", "path to the cluster template file.")
	ensure.NoError(cmd.MarkPersistentFlagRequired("file"))
}
