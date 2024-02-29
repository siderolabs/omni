// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package template

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/siderolabs/omni/client/pkg/template/operations"
)

// renderCmd represents the template render command.
var renderCmd = &cobra.Command{
	Use:     "render",
	Short:   "Render a cluster template to a set of resources.",
	Long:    `Validate template contents, convert to resources and output resources to stdout as YAML. This command is offline (doesn't access API).`,
	Example: "",
	Args:    cobra.NoArgs,
	RunE: func(*cobra.Command, []string) error {
		return render()
	},
}

func render() error {
	f, err := os.Open(cmdFlags.TemplatePath)
	if err != nil {
		return err
	}

	defer f.Close() //nolint:errcheck

	return operations.RenderTemplate(f, os.Stdout)
}

func init() {
	addRequiredFileFlag(renderCmd)
	templateCmd.AddCommand(renderCmd)
}
