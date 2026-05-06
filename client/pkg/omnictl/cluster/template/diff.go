// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package template

import (
	"context"
	"errors"
	"os"

	"github.com/spf13/cobra"

	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/execdiff"
	"github.com/siderolabs/omni/client/pkg/omnictl/internal/access"
	"github.com/siderolabs/omni/client/pkg/template/operations"
)

// diffCmd represents the template diff command.
var diffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Show diff in resources if the template is synced.",
	Long: `Query existing resources for the cluster and compare them with the resources generated from the template. This command requires API access.

` + execdiff.EnvExternalDiff + ` environment variable can be used to select an
external diff command. Users can use external commands with params too,
example: ` + execdiff.EnvExternalDiff + `="colordiff -N -u"

By default, the built-in colorized unified diff is used.

Exit status: 0 No differences were found. 1 Differences were found. >1 An error occurred.`,
	Example: "",
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		differ := execdiff.New(os.Stdout)

		runErr := access.WithClient(func(ctx context.Context, c *client.Client, _ access.ServerInfo) error {
			return diff(ctx, c, differ)
		})

		flushed, flushErr := differ.Flush()
		if flushErr != nil {
			return errors.Join(runErr, flushErr)
		}

		if runErr != nil {
			return runErr
		}

		if flushed {
			cmd.SilenceErrors = true
			cmd.SilenceUsage = true

			return execdiff.ErrDifferencesFound
		}

		return nil
	},
}

func diff(ctx context.Context, client *client.Client, differ *execdiff.Differ) error {
	f, err := os.Open(cmdFlags.TemplatePath)
	if err != nil {
		return err
	}

	defer f.Close() //nolint:errcheck

	return operations.DiffTemplate(ctx, f, os.Stdout, client.Omni().State(), differ, resolvedRoot)
}

func init() {
	addRequiredFileFlag(diffCmd)
	templateCmd.AddCommand(diffCmd)
}
