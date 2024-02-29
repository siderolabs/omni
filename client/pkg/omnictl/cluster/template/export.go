// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package template

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/siderolabs/gen/ensure"
	"github.com/spf13/cobra"

	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/omnictl/internal/access"
	"github.com/siderolabs/omni/client/pkg/template/operations"
)

var exportCmdFlags struct {
	cluster string
	output  string
	force   bool
}

// exportCmd represents the template export command.
var exportCmd = &cobra.Command{
	Use:   "export cluster-name",
	Short: "Export a cluster template from an existing cluster on Omni.",
	Long:  `Export a cluster template from an existing cluster on Omni. This command requires API access.`,
	Args:  cobra.NoArgs,
	RunE: func(*cobra.Command, []string) error {
		return access.WithClient(export)
	},
}

func export(ctx context.Context, client *client.Client) (err error) {
	output := os.Stdout

	if exportCmdFlags.output != "" {
		var openErr error

		flags := os.O_WRONLY | os.O_CREATE | os.O_EXCL
		if exportCmdFlags.force {
			flags = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
		}

		output, openErr = os.OpenFile(exportCmdFlags.output, flags, 0o644)
		if openErr != nil {
			return fmt.Errorf("failed to open output file: %w", openErr)
		}

		defer func() { err = errors.Join(err, output.Close()) }()
	}

	_, err = operations.ExportTemplate(ctx, client.Omni().State(), exportCmdFlags.cluster, output)

	return err
}

func init() {
	exportCmd.Flags().StringVarP(&exportCmdFlags.cluster, "cluster", "c", "", "cluster name")
	exportCmd.Flags().StringVarP(&exportCmdFlags.output, "output", "o", "", "output file (default: stdout)")
	exportCmd.Flags().BoolVarP(&exportCmdFlags.force, "force", "f", false, "overwrite output file if it exists")

	ensure.NoError(exportCmd.MarkFlagRequired("cluster"))

	templateCmd.AddCommand(exportCmd)
}
