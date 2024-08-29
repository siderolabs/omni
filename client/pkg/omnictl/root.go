// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package omnictl

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/siderolabs/omni/client/pkg/compression"
	"github.com/siderolabs/omni/client/pkg/omnictl/config"
	"github.com/siderolabs/omni/client/pkg/omnictl/internal/access"
)

// RootCmd represents the base command when called without any subcommands.
var RootCmd = &cobra.Command{
	Use:               "omnictl",
	Short:             "A CLI for accessing Omni API.",
	Long:              ``,
	SilenceUsage:      true,
	DisableAutoGenTag: true,
	PersistentPreRunE: func(*cobra.Command, []string) error {
		return compression.InitConfig(true)
	},
}

func init() {
	RootCmd.PersistentFlags().StringVar(&access.CmdFlags.Omniconfig, "omniconfig", "",
		fmt.Sprintf("The path to the omni configuration file. Defaults to '%s' env variable if set, otherwise the config directory according to the XDG specification.",
			config.OmniConfigEnvVar,
		))
	RootCmd.PersistentFlags().StringVar(&access.CmdFlags.Context, "context", "",
		"The context to be used. Defaults to the selected context in the omniconfig file.")
	RootCmd.PersistentFlags().BoolVar(&access.CmdFlags.InsecureSkipTLSVerify, "insecure-skip-tls-verify", false,
		"Skip TLS verification for the Omni GRPC and HTTP API endpoints.")
}
