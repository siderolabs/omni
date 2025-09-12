// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster

import (
	"context"
	"fmt"
	"os"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/spf13/cobra"

	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omnictl/internal/access"
)

var abortImportCmd = &cobra.Command{
	Use:   "abort <cluster name>",
	Short: "Abort an ongoing cluster import operation",
	Long: `Abort an ongoing cluster import operation. This will clean up any resources created during the import process and 
will only work if the cluster is locked and tainted as "importing"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return access.WithClient(func(ctx context.Context, client *client.Client) error {
			return abortImport(ctx, client, args[0])
		})
	},
}

func abortImport(ctx context.Context, client *client.Client, id resource.ID) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	clusterMD := omni.NewCluster(resources.DefaultNamespace, id).Metadata()

	cluster, err := client.Omni().State().Get(ctx, clusterMD)
	if err != nil {
		return err
	}

	clusterStatus, err := client.Omni().State().Get(ctx, omni.NewClusterStatus(resources.DefaultNamespace, id).Metadata())
	if err != nil {
		return err
	}

	if _, ok := clusterStatus.Metadata().Labels().Get(omni.LabelClusterTaintedByImporting); !ok {
		return fmt.Errorf("cluster %q is not tainted as importing", id)
	}

	if _, ok := cluster.Metadata().Annotations().Get(omni.ClusterLocked); !ok {
		return fmt.Errorf("cluster %q is not locked", id)
	}

	fmt.Fprintf(os.Stdout, "Aborting import operation for cluster %q\n", id) //nolint:errcheck

	_, err = client.Management().TearDownLockedCluster(ctx, id)
	if err != nil {
		return err
	}

	_, err = client.Omni().State().WatchFor(ctx, clusterMD, state.WithEventTypes(state.Destroyed))
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stdout, "Import operation was aborted successfully for cluster %q\n", id) //nolint:errcheck

	return nil
}

// importCmd represents the cluster import commands.
var importCmd = &cobra.Command{
	Use:     "import",
	Short:   "Cluster import related commands.",
	Long:    `Commands to manage cluster import operation.`,
	Hidden:  true, // Hidden until we have a proper import flow.
	Example: "",
}

func init() {
	clusterCmd.AddCommand(importCmd)
	importCmd.AddCommand(abortImportCmd)
}
