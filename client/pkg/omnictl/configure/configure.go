// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package configure contains omnictl configure commands.
package configure

import (
	"github.com/spf13/cobra"
)

// configureCmd represents the configure sub-command.
var configureCmd = &cobra.Command{
	Use:     "configure",
	Aliases: []string{"c"},
	Short:   "Configuration subcommands.",
	Example: "",
}

// RootCmd exposes root configure command.
func RootCmd() *cobra.Command {
	return configureCmd
}
