// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package kubernetes contains commands related to cluster Kubernetes operations.
package kubernetes

import (
	"github.com/spf13/cobra"
)

// kubernetesCmd represents the kubernetes sub-command.
var kubernetesCmd = &cobra.Command{
	Use:     "kubernetes",
	Aliases: []string{"k"},
	Short:   "Cluster Kubernetes management subcommands.",
	Long:    `Commands to render, validate, manage cluster templates.`,
	Example: "",
}

// RootCmd exports kubernetesCmd.
func RootCmd() *cobra.Command {
	return kubernetesCmd
}
