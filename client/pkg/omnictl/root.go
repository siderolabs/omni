// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package omnictl

import (
	"fmt"
	"path/filepath"

	"github.com/siderolabs/talos/pkg/machinery/constants"
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
		fmt.Sprintf("The path to the omni configuration file. Defaults to '%s' env variable if set, otherwise '%s'. "+
			"'%s' is Deprecated and only used as a last resort for reading existing configuration file.",
			config.OmniConfigEnvVar,
			filepath.Join("$HOME", constants.TalosDir, config.OmniRelativePath),
			filepath.Join("$XDG_CONFIG_HOME", config.OmniRelativePath),
		))
	RootCmd.PersistentFlags().StringVar(&access.CmdFlags.Context, "context", "",
		"The context to be used. Defaults to the selected context in the omniconfig file.")
	RootCmd.PersistentFlags().BoolVar(&access.CmdFlags.InsecureSkipTLSVerify, "insecure-skip-tls-verify", false,
		"Skip TLS verification for the Omni GRPC and HTTP API endpoints.")
	RootCmd.PersistentFlags().StringVar(
		&access.CmdFlags.SideroV1KeysDir,
		"siderov1-keys-dir",
		"",
		fmt.Sprintf("The path to the SideroV1 auth PGP keys directory. Defaults to '%s' env variable if set, otherwise '%s'.",
			constants.SideroV1KeysDirEnvVar,
			filepath.Join("$HOME", constants.TalosDir, constants.SideroV1KeysDir),
		),
	)
}
