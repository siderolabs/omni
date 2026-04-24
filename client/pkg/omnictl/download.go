// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package omnictl

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/client/pkg/omnictl/internal/access"
	"github.com/siderolabs/omni/client/pkg/omnictl/internal/download"
)

var deprecatedDownloadFlags struct {
	architecture            string
	output                  string
	talosVersion            string
	labels                  []string
	extraKernelArgs         []string
	extensions              []string
	pxe                     bool
	secureBoot              bool
	useSiderolinkGRPCTunnel bool
}

func init() {
	downloadCmd.Flags().BoolVar(&deprecatedDownloadFlags.pxe, "pxe", false, "Print PXE URL and exit")
	downloadCmd.Flags().BoolVar(&deprecatedDownloadFlags.secureBoot, "secureboot", false, "Download SecureBoot enabled installation media")
	downloadCmd.Flags().StringVar(&deprecatedDownloadFlags.architecture, "arch", "amd64", "Image architecture to download (amd64, arm64)")
	downloadCmd.Flags().StringVar(&deprecatedDownloadFlags.output, "output", ".", "Output file or directory, defaults to current working directory")
	downloadCmd.Flags().StringVar(&deprecatedDownloadFlags.talosVersion, "talos-version", constants.DefaultTalosVersion, "Talos version to be used in the generated installation media")
	downloadCmd.Flags().StringSliceVar(&deprecatedDownloadFlags.labels, "initial-labels", nil, "Bake initial labels into the generated installation media")
	downloadCmd.Flags().StringArrayVar(&deprecatedDownloadFlags.extraKernelArgs, "extra-kernel-args", nil, "Add extra kernel args to the generated installation media")
	downloadCmd.Flags().StringSliceVar(&deprecatedDownloadFlags.extensions, "extensions", nil, "Generate installation media with extensions pre-installed")
	downloadCmd.Flags().BoolVar(&deprecatedDownloadFlags.useSiderolinkGRPCTunnel, "use-siderolink-grpc-tunnel", false,
		"Configure Talos to use the SideroLink (WireGuard) gRPC tunnel over HTTP/2 for Omni management traffic, instead of UDP."+
			" Only enable this if the network blocks UDP packets, as HTTP tunneling adds significant overhead for communications.")

	RootCmd.AddCommand(downloadCmd)
}

// downloadCmd represents the deprecated download command.
var downloadCmd = &cobra.Command{
	Use:        "download <image name>",
	Short:      "Download installer media",
	Deprecated: "use 'omnictl media download <preset-name>' instead",
	Long: `This command downloads installer media from the server

It accepts one argument, which is the name of the image to download. Name can be one of the following:

     * iso - downloads the latest ISO image
     * AWS AMI (amd64), Vultr (arm64), Raspberry Pi 4 Model B - full image name
     * oracle, aws, vmware - platform name
     * rpi_generic, rockpi_4c, rock64 - board name

To get the full list of available images, look at the output of the following command:
    omnictl get installationmedia -o yaml

The download command tries to match the passed string in this order:

    * name
    * profile

By default it will download amd64 image if there are multiple images available for the same name.

For example, to download the latest ISO image for arm64, run:

    omnictl download iso --arch amd64

To download the same ISO with two extensions added, the --extensions argument gets repeated to produce a stringArray:

    omnictl download iso --arch amd64 --extensions intel-ucode --extensions qemu-guest-agent

To download the latest Vultr image, run:

    omnictl download "vultr"

To download the latest Radxa ROCK PI 4 image, run:

    omnictl download "rpi_generic"
`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return access.WithClient(func(ctx context.Context, client *client.Client, _ access.ServerInfo) error {
			if args[0] == "" {
				return fmt.Errorf("image name is required")
			}

			output, err := filepath.Abs(deprecatedDownloadFlags.output)
			if err != nil {
				return err
			}

			if err = download.MakePath(output); err != nil {
				return err
			}

			params := download.Params{
				Architecture:    deprecatedDownloadFlags.architecture,
				Output:          output,
				TalosVersion:    deprecatedDownloadFlags.talosVersion,
				Labels:          deprecatedDownloadFlags.labels,
				ExtraKernelArgs: deprecatedDownloadFlags.extraKernelArgs,
				Extensions:      deprecatedDownloadFlags.extensions,
				PXE:             deprecatedDownloadFlags.pxe,
				SecureBoot:      deprecatedDownloadFlags.secureBoot,
				GrpcTunnelMode:  download.GRPCTunnelModeFromFlag(cmd, "use-siderolink-grpc-tunnel", deprecatedDownloadFlags.useSiderolinkGRPCTunnel),
			}

			image, err := download.FindImage(ctx, client, args[0], params)
			if err != nil {
				return err
			}

			return download.DownloadImageTo(ctx, client, image, params)
		})
	},
	ValidArgsFunction: func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return download.ImageCompletion(deprecatedDownloadFlags.architecture, toComplete)
	},
}
