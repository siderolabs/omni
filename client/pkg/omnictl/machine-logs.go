// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package omnictl

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/omnictl/internal/access"
	"github.com/siderolabs/omni/client/pkg/omnictl/logformat"
)

var logsCmdFlags struct {
	logFormat string
	follow    bool
	tailLines int32
}

// getCmd represents the get logs command.
var logsCmd = &cobra.Command{
	Use:     "machine-logs machineID",
	Aliases: []string{"l"},
	Short:   "Get logs for a machine",
	Long:    `Get logs for a provided machine id`,
	Example: "",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return access.WithClient(getLogs(cmd, args))
	},
}

func getLogs(_ *cobra.Command, args []string) func(ctx context.Context, client *client.Client) error {
	return func(ctx context.Context, client *client.Client) error {
		machineID := args[0]

		logReader, err := client.Management().LogsReader(ctx, machineID, logsCmdFlags.follow, logsCmdFlags.tailLines)
		if err != nil {
			return fmt.Errorf("failed to get logs stream for '%s': %w", machineID, err)
		}

		switch logsCmdFlags.logFormat {
		case "omni":
			err = logformat.NewOmniOutput(logReader).Run(ctx)
		case "dmesg":
			err = logformat.NewDmesgOutput(logReader).Run(ctx)
		default:
			err = logformat.NewRawOutput(logReader).Run()
		}

		if err != nil {
			return fmt.Errorf("failed to print logs for '%s': %w", machineID, err)
		}

		return nil
	}
}

func init() {
	logsCmd.Flags().BoolVarP(&logsCmdFlags.follow, "follow", "f", false, "specify if the logs should be streamed")
	logsCmd.Flags().Int32Var(&logsCmdFlags.tailLines, "tail", -1, "lines of log file to display (default is to show from the beginning)")
	logsCmd.Flags().StringVar(&logsCmdFlags.logFormat, "log-format", "raw", "log format (raw, omni, dmesg) to display (default is to display in raw format)")
	RootCmd.AddCommand(logsCmd)
}
