// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/spf13/cobra"

	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/clusterimport"
	"github.com/siderolabs/omni/client/pkg/omnictl/internal/access"
)

const (
	flagImportForce = "force"
	flagNodes       = "nodes"
)

var abortImportCmd = &cobra.Command{
	Use:   "abort <cluster name>",
	Short: "Abort an ongoing cluster import operation",
	Long: `Abort an ongoing cluster import operation. This will clean up any resources created during the import process and 
will only work if the cluster is locked and tainted as "importing"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return access.WithClient(func(ctx context.Context, client *client.Client) error {
			clusterID := args[0]
			if clusterID == "" {
				return fmt.Errorf("cluster name is required")
			}

			omniState := client.Omni().State()

			if err := clusterimport.Abort(ctx, omniState, clusterID, os.Stderr); err != nil {
				return fmt.Errorf("failed to abort import operation for cluster %q: %w", clusterID, err)
			}

			fmt.Fprintf(os.Stderr, "import operation was aborted successfully for cluster %q\n", clusterID) //nolint:errcheck

			return nil
		})
	},
}

var input clusterimport.Input

func importCluster(ctx context.Context, client *client.Client) error {
	ctx, cancel := context.WithTimeout(ctx, importCmdFlags.waitTimeout)
	defer cancel()

	input.LogWriter = os.Stderr

	if len(input.Nodes) == 0 {
		return fmt.Errorf("at least one node is required to import a cluster")
	}

	omniState := client.Omni().State()

	imageFactoryClient, err := clusterimport.BuildImageFactoryClient(ctx, omniState)
	if err != nil {
		return err
	}

	talosClient, err := clusterimport.BuildTalosClient(ctx, importCmdFlags.talosConfig, importCmdFlags.talosContext, access.CmdFlags.SideroV1KeysDir, importCmdFlags.talosEndpoints)
	if err != nil {
		return err
	}

	importContext, err := clusterimport.BuildContext(ctx, input, omniState, imageFactoryClient, talosClient)
	if err != nil {
		return err
	}

	defer importContext.Close() //nolint:errcheck

	if err = importContext.Run(ctx); err != nil {
		if !errors.Is(err, clusterimport.ErrValidation) {
			return fmt.Errorf("failed to import cluster %q: %w", importContext.ClusterID, err)
		}

		return fmt.Errorf("failed to validate cluster status %q, can be overridden with --%s:  %w", importContext.ClusterID, flagImportForce, err)
	}

	fmt.Fprintf(os.Stderr, "cluster %q is imported successfully but marked as 'locked' to prevent changes done by Omni\n", importContext.ClusterID)

	return nil
}

var importCmdFlags struct {
	talosConfig    string
	talosContext   string
	talosEndpoints []string
	waitTimeout    time.Duration
}

// importCmd represents the cluster import commands.
var importCmd = &cobra.Command{
	Use:     "import",
	Short:   "Cluster import related commands.",
	Long:    `Commands to manage cluster import operation.`,
	Example: "",
	RunE: func(_ *cobra.Command, _ []string) error {
		return access.WithClient(importCluster)
	},
}

func init() {
	clusterCmd.AddCommand(importCmd)
	importCmd.AddCommand(abortImportCmd)
	importCmd.Flags().StringSliceVarP(&input.Nodes, flagNodes, "n", []string{}, "endpoints of all nodes to import")
	importCmd.MarkFlagRequired(flagNodes) //nolint:errcheck

	importCmd.Flags().StringVar(
		&importCmdFlags.talosConfig,
		"talosconfig",
		"",
		fmt.Sprintf("The path to the Talos configuration file. Defaults to '%s' env variable if set, otherwise '%s'.",
			constants.TalosConfigEnvVar,
			filepath.Join("$HOME", constants.TalosDir, constants.TalosconfigFilename),
		),
	)
	importCmd.Flags().StringSliceVarP(&importCmdFlags.talosEndpoints, "talos-endpoints", "", []string{}, "override default endpoints in Talos configuration file")
	importCmd.Flags().StringVarP(&importCmdFlags.talosContext, "talos-context", "", "",
		"the context to be used for accessing talos. defaults to the selected context in the Talos configuration file")
	importCmd.Flags().StringVarP(&input.Versions.TalosVersion, "talos-version", "", "",
		"talos version of the cluster, if not set, will be detected from the nodes")
	importCmd.Flags().StringVarP(&input.Versions.KubernetesVersion, "kubernetes-version", "", "",
		"kubernetes version of the cluster, if not set, will be detected from the nodes")
	importCmd.Flags().StringVarP(&input.Versions.InitialTalosVersion, "initial-talos-version", "", "",
		"initial talos version used on cluster creation, if not set current talos version will be used")
	importCmd.Flags().StringVarP(&input.Versions.InitialKubernetesVersion, "initial-kubernetes-version", "", "",
		"initial kubernetes version used on cluster creation, if not set current kubernetes version will be used")
	importCmd.Flags().BoolVarP(&input.DryRun, "dry-run", "d", false, "skip the actual import and show the import plan instead")
	importCmd.Flags().BoolVar(&input.Force, flagImportForce, false, "force import even if validations fail")
	importCmd.Flags().BoolVar(&input.SkipHealthCheck, "skip-health-check", false, "skip performing cluster health check before import")
	importCmd.Flags().StringVarP(&input.BackupOutput, "backup-output", "O", "", "backup file for storing node machine configs before import")
	importCmd.Flags().DurationVar(&importCmdFlags.waitTimeout, "wait-timeout", 5*time.Minute, "timeout to wait for the cluster import to complete")
}
