// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package cluster contains commands related to cluster operations.
package cluster

import (
	"github.com/spf13/cobra"

	"github.com/siderolabs/omni/client/pkg/omnictl/cluster/kubernetes"
	"github.com/siderolabs/omni/client/pkg/omnictl/cluster/template"
)

// clusterCmd represents the cluster sub-command.
var clusterCmd = &cobra.Command{
	Use:     "cluster",
	Aliases: []string{"c"},
	Short:   "Cluster-related subcommands.",
	Long:    `Commands to destroy clusters and manage cluster templates.`,
	Example: "",
}

// RootCmd exposes root cluster command.
func RootCmd() *cobra.Command {
	return clusterCmd
}

func init() {
	clusterCmd.AddCommand(template.RootCmd())
	clusterCmd.AddCommand(kubernetes.RootCmd())
}
