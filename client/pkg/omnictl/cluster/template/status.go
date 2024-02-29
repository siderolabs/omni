// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package template

import (
	"context"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/omnictl/internal/access"
	"github.com/siderolabs/omni/client/pkg/template/operations"
)

var statusCmdFlags struct {
	options operations.StatusOptions
	wait    time.Duration
}

// statusCmd represents the cluster status command.
var statusCmd = &cobra.Command{
	Use:     "status",
	Short:   "Show template cluster status, wait for the cluster to be ready.",
	Long:    `Shows current cluster status, if the terminal supports it, watch the status as it updates. The command waits for the cluster to be ready by default.`,
	Example: "",
	Args:    cobra.NoArgs,
	RunE: func(*cobra.Command, []string) error {
		return access.WithClient(status)
	},
}

func status(ctx context.Context, client *client.Client) error {
	f, err := os.Open(cmdFlags.TemplatePath)
	if err != nil {
		return err
	}

	defer f.Close() //nolint:errcheck

	if statusCmdFlags.wait > 0 {
		var cancel context.CancelFunc

		ctx, cancel = context.WithTimeout(ctx, statusCmdFlags.wait)
		defer cancel()

		statusCmdFlags.options.Wait = true
	} else {
		statusCmdFlags.options.Wait = false
	}

	return operations.StatusTemplate(ctx, f, os.Stdout, client.Omni().State(), statusCmdFlags.options)
}

func init() {
	addRequiredFileFlag(statusCmd)
	statusCmd.PersistentFlags().BoolVarP(&statusCmdFlags.options.Quiet, "quiet", "q", false, "suppress output")
	statusCmd.PersistentFlags().DurationVarP(&statusCmdFlags.wait, "wait", "w", 5*time.Minute, "wait timeout, if zero, report current status and exit")
	templateCmd.AddCommand(statusCmd)
}
