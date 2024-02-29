// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package template

import (
	"context"
	"os"

	"github.com/spf13/cobra"

	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/omnictl/internal/access"
	"github.com/siderolabs/omni/client/pkg/template/operations"
)

var syncCmdFlags struct {
	options operations.SyncOptions
}

// syncCmd represents the template sync command.
var syncCmd = &cobra.Command{
	Use:     "sync",
	Short:   "Apply template to the Omni.",
	Long:    `Query existing resources for the cluster and compare them with the resources generated from the template, create/update/delete resources as needed. This command requires API access.`,
	Example: "",
	Args:    cobra.NoArgs,
	RunE: func(*cobra.Command, []string) error {
		return access.WithClient(sync)
	},
}

func sync(ctx context.Context, client *client.Client) error {
	f, err := os.Open(cmdFlags.TemplatePath)
	if err != nil {
		return err
	}

	defer f.Close() //nolint:errcheck

	return operations.SyncTemplate(ctx, f, os.Stdout, client.Omni().State(), syncCmdFlags.options)
}

func init() {
	addRequiredFileFlag(syncCmd)
	syncCmd.PersistentFlags().BoolVarP(&syncCmdFlags.options.Verbose, "verbose", "v", false, "verbose output (show diff for each resource)")
	syncCmd.PersistentFlags().BoolVarP(&syncCmdFlags.options.DryRun, "dry-run", "d", false, "dry run")
	templateCmd.AddCommand(syncCmd)
}
