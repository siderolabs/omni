// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package installationmedia provides the `media` command group for omnictl.
package installationmedia

import "github.com/spf13/cobra"

const (
	flagArch            = "arch"
	flagTalosVersion    = "talos-version"
	flagInitialLabels   = "initial-labels"
	flagExtraKernelArgs = "extra-kernel-args"
	flagExtensions      = "extensions"
	flagSecureBoot      = "secureboot"
	flagPlatform        = "platform"
	flagOverlay         = "overlay"
	flagOverlayOptions  = "overlay-options"
	flagJoinTokenName   = "join-token"
	flagBootloader      = "bootloader"
	flagGRPCTunnel      = "use-siderolink-grpc-tunnel"
	flagFormat          = "format"
)

var cmd = &cobra.Command{
	Use:     "media",
	Aliases: []string{"installation-media", "im"},
	Short:   "Manage installation media presets.",
	Long:    "Commands to create, list, delete, and download from installation media presets.",
}

// RootCmd returns the root command for the media group.
func RootCmd() *cobra.Command {
	return cmd
}
