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

// diffCmd represents the template diff command.
var diffCmd = &cobra.Command{
	Use:     "diff",
	Short:   "Show diff in resources if the template is synced.",
	Long:    `Query existing resources for the cluster and compare them with the resources generated from the template. This command requires API access.`,
	Example: "",
	Args:    cobra.NoArgs,
	RunE: func(*cobra.Command, []string) error {
		return access.WithClient(diff)
	},
}

func diff(ctx context.Context, client *client.Client) error {
	f, err := os.Open(cmdFlags.TemplatePath)
	if err != nil {
		return err
	}

	defer f.Close() //nolint:errcheck

	return operations.DiffTemplate(ctx, f, os.Stdout, client.Omni().State())
}

func init() {
	addRequiredFileFlag(diffCmd)
	templateCmd.AddCommand(diffCmd)
}
