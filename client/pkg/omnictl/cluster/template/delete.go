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

var deleteCmdFlags struct {
	options operations.SyncOptions
}

// deleteCmd represents the template delete command.
var deleteCmd = &cobra.Command{
	Use:     "delete",
	Short:   "Delete all cluster template resources from Omni.",
	Long:    `Delete all resources related to the cluster template. This command requires API access.`,
	Example: "",
	Args:    cobra.NoArgs,
	RunE: func(*cobra.Command, []string) error {
		return access.WithClient(deleteImpl)
	},
}

func deleteImpl(ctx context.Context, client *client.Client) error {
	f, err := os.Open(cmdFlags.TemplatePath)
	if err != nil {
		return err
	}

	defer f.Close() //nolint:errcheck

	return operations.DeleteTemplate(ctx, f, os.Stdout, client.Omni().State(), deleteCmdFlags.options)
}

func init() {
	addRequiredFileFlag(deleteCmd)
	deleteCmd.PersistentFlags().BoolVarP(&deleteCmdFlags.options.Verbose, "verbose", "v", false, "verbose output (show diff for each resource)")
	deleteCmd.PersistentFlags().BoolVarP(&deleteCmdFlags.options.DryRun, "dry-run", "d", false, "dry run")
	deleteCmd.PersistentFlags().BoolVar(&deleteCmdFlags.options.DestroyMachines, "destroy-disconnected-machines", false, "removes all disconnected machines which are part of the cluster from Omni")
	templateCmd.AddCommand(deleteCmd)
}
