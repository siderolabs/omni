// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package cluster contains commands related to cluster operations.
package cluster

import (
	"context"
	"fmt"
	"os"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/spf13/cobra"

	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omnictl/cluster/kubernetes"
	"github.com/siderolabs/omni/client/pkg/omnictl/cluster/secret"
	"github.com/siderolabs/omni/client/pkg/omnictl/cluster/template"
	"github.com/siderolabs/omni/client/pkg/omnictl/internal/access"
)

var lockClusterCmd = &cobra.Command{
	Use:   "lock cluster-id",
	Short: "Lock the cluster",
	Long:  `When locked, no config updates, upgrades and downgrades will be performed on the cluster nodes.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		return access.WithClient(setClusterLocked(args[0], true))
	},
}

var unlockClusterCmd = &cobra.Command{
	Use:   "unlock cluster-id",
	Short: "Unlock the cluster",
	Long:  `Removes locked annotation from the cluster.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		return access.WithClient(setClusterLocked(args[0], false))
	},
}

func setClusterLocked(clusterID resource.ID, lock bool) func(context.Context, *client.Client) error {
	return func(ctx context.Context, client *client.Client) error {
		st := client.Omni().State()

		cluster, err := safe.StateGet[*omni.Cluster](ctx, st, omni.NewCluster(clusterID).Metadata())
		if err != nil {
			return err
		}

		_, err = safe.StateUpdateWithConflicts(ctx, st, cluster.Metadata(), func(res *omni.Cluster) error {
			if lock {
				res.Metadata().Annotations().Set(omni.ClusterLocked, "")
			} else {
				res.Metadata().Annotations().Delete(omni.ClusterLocked)
				res.Metadata().Annotations().Delete(omni.ClusterImportIsInProgress)
			}

			return nil
		})
		if err != nil {
			return err
		}

		fmt.Fprintf(os.Stderr, "cluster %q lock status: %t\n", clusterID, lock)

		return nil
	}
}

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
	clusterCmd.AddCommand(lockClusterCmd)
	clusterCmd.AddCommand(unlockClusterCmd)
	clusterCmd.AddCommand(secret.RootCmd())
}
