// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster

import (
	"context"
	"os"

	"github.com/spf13/cobra"

	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/omnictl/internal/access"
	"github.com/siderolabs/omni/client/pkg/template/operations"
)

var deleteCmdFlags struct {
	options operations.SyncOptions
}

// deleteCmd represents the cluster delete command.
var deleteCmd = &cobra.Command{
	Use:     "delete cluster-name",
	Short:   "Delete all cluster resources.",
	Long:    `Delete all resources related to the cluster. The command waits for the cluster to be fully destroyed.`,
	Example: "",
	Args:    cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		return access.WithClient(deleteImpl(args[0]))
	},
}

func deleteImpl(clusterName string) func(ctx context.Context, client *client.Client) error {
	return func(ctx context.Context, client *client.Client) error {
		return operations.DeleteCluster(ctx, clusterName, os.Stdout, client.Omni().State(), deleteCmdFlags.options)
	}
}

func init() {
	deleteCmd.PersistentFlags().BoolVarP(&deleteCmdFlags.options.Verbose, "verbose", "v", false, "verbose output (show diff for each resource)")
	deleteCmd.PersistentFlags().BoolVarP(&deleteCmdFlags.options.DryRun, "dry-run", "d", false, "dry run")
	deleteCmd.PersistentFlags().BoolVar(&deleteCmdFlags.options.DestroyMachines, "destroy-disconnected-machines", false, "removes all disconnected machines which are part of the cluster from Omni")
	clusterCmd.AddCommand(deleteCmd)
}
