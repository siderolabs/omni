// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package template

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/siderolabs/omni/client/pkg/template/operations"
)

// validateCmd represents the template validate command.
var validateCmd = &cobra.Command{
	Use:     "validate",
	Short:   "Validate a cluster template.",
	Long:    `Validate that template contains valid structures, and there are no other warnings. This command is offline (doesn't access API).`,
	Example: "",
	Args:    cobra.NoArgs,
	RunE: func(*cobra.Command, []string) error {
		return validate()
	},
}

func validate() error {
	f, err := os.Open(cmdFlags.TemplatePath)
	if err != nil {
		return err
	}

	defer f.Close() //nolint:errcheck

	return operations.ValidateTemplate(f)
}

func init() {
	addRequiredFileFlag(validateCmd)
	templateCmd.AddCommand(validateCmd)
}
