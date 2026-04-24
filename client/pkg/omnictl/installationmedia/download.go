// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package installationmedia

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/spf13/cobra"

	"github.com/siderolabs/omni/client/api/omni/management"
	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omnictl/internal/access"
	"github.com/siderolabs/omni/client/pkg/omnictl/internal/download"
)

var downloadCmdFlags struct {
	overlay                 string
	platform                string
	arch                    string
	output                  string
	talosVersion            string
	format                  string
	bootloader              string
	joinTokenName           string
	overlayOptions          string
	labels                  []string
	extensions              []string
	extraKernelArgs         []string
	secureBoot              bool
	useSiderolinkGRPCTunnel bool
}

var downloadCmd = &cobra.Command{
	Use:   "download <name> [flags]",
	Short: "Download installation media from a preset.",
	Long: `Download installation media using a saved preset configuration.

Any flag provided here overrides the corresponding value from the preset.

The format flag determines the output file type for metal presets:
    * iso   - ISO image (default)
    * raw   - Raw disk image (.raw.xz)
    * qcow2 - QEMU disk image (.qcow2)
    * pxe   - Print PXE boot URL (no file downloaded)

For cloud presets, the format is automatically determined by the platform.
For SBC presets, a raw disk image is always produced.

Examples:
    # Download from a preset (format is auto-detected from the preset type)
    omnictl media download my-preset

    # Force raw disk image for a metal preset
    omnictl media download my-preset --format raw

    # Get PXE boot URL for a metal preset
    omnictl media download my-preset --format pxe

    # Override arch and extensions
    omnictl media download my-preset --arch arm64 --extensions qemu-guest-agent

    # Download to a specific directory
    omnictl media download my-preset --output /tmp/images/
`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return access.WithClient(func(ctx context.Context, client *client.Client, info access.ServerInfo) error {
			if !info.ServerSupports(1, 7) {
				return fmt.Errorf("installation media presets require Omni v1.7.0 or newer (server is %s)", info.Version)
			}

			output, err := filepath.Abs(downloadCmdFlags.output)
			if err != nil {
				return err
			}

			if err = download.MakePath(output); err != nil {
				return err
			}

			return runDownload(ctx, cmd, client, args[0], output)
		})
	},
	ValidArgsFunction: func(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		return presetCompletion(toComplete)
	},
}

func init() {
	downloadCmd.Flags().StringVar(&downloadCmdFlags.arch, flagArch, "amd64", "Image architecture to download (amd64, arm64)")
	downloadCmd.Flags().StringVar(&downloadCmdFlags.output, "output", ".", "Output file or directory, defaults to current working directory")
	downloadCmd.Flags().StringVar(&downloadCmdFlags.talosVersion, flagTalosVersion, constants.DefaultTalosVersion, "Talos version to be used in the generated installation media")
	downloadCmd.Flags().StringSliceVar(&downloadCmdFlags.labels, flagInitialLabels, nil, "Bake initial labels into the generated installation media")
	downloadCmd.Flags().StringArrayVar(&downloadCmdFlags.extraKernelArgs, flagExtraKernelArgs, nil, "Add extra kernel args to the generated installation media")
	downloadCmd.Flags().StringSliceVar(&downloadCmdFlags.extensions, flagExtensions, nil, "Generate installation media with extensions pre-installed")
	downloadCmd.Flags().BoolVar(&downloadCmdFlags.secureBoot, flagSecureBoot, false, "Download SecureBoot enabled installation media")
	downloadCmd.Flags().BoolVar(&downloadCmdFlags.useSiderolinkGRPCTunnel, flagGRPCTunnel, false,
		"Configure Talos to use the SideroLink (WireGuard) gRPC tunnel over HTTP/2 for Omni management traffic, instead of UDP."+
			" Only enable this if the network blocks UDP packets, as HTTP tunneling adds significant overhead for communications.")
	downloadCmd.Flags().StringVar(&downloadCmdFlags.platform, flagPlatform, "", "Cloud platform override (e.g., aws, gcp, azure, vultr)")
	downloadCmd.Flags().StringVar(&downloadCmdFlags.overlay, flagOverlay, "", "SBC overlay name override (e.g., rpi_generic, rockpi_4c)")
	downloadCmd.Flags().StringVar(&downloadCmdFlags.overlayOptions, flagOverlayOptions, "", "SBC overlay options (YAML string)")
	downloadCmd.Flags().StringVar(&downloadCmdFlags.joinTokenName, flagJoinTokenName, "", "Join token ID override (uses preset token if not specified)")
	downloadCmd.Flags().StringVar(&downloadCmdFlags.bootloader, flagBootloader, "auto", "Bootloader type (auto, uefi, bios, dual)")
	downloadCmd.Flags().StringVar(&downloadCmdFlags.format, flagFormat, "", "Output format for metal presets: iso, raw, qcow2, pxe (auto-detected if omitted)")

	cmd.AddCommand(downloadCmd)
}

//nolint:gocyclo,cyclop
func runDownload(ctx context.Context, cmd *cobra.Command, client *client.Client, presetName, output string) error {
	preset, err := safe.ReaderGetByID[*omni.InstallationMediaConfig](ctx, client.Omni().State(), presetName)
	if err != nil {
		return fmt.Errorf("failed to get installation media preset %q: %w", presetName, err)
	}

	spec := preset.TypedSpec().Value

	arch := download.ArchToString(spec.Architecture)
	if cmd.Flags().Changed(flagArch) {
		arch = downloadCmdFlags.arch
	}

	mediaBuildOpts := download.MediaBuildOptions{
		Format: downloadCmdFlags.format,
	}

	hasOverlayOptions := cmd.Flags().Changed(flagOverlayOptions)

	switch {
	case cmd.Flags().Changed(flagOverlay):
		mediaBuildOpts.Overlay = downloadCmdFlags.overlay
		mediaBuildOpts.OverlayOptions = downloadCmdFlags.overlayOptions
	case cmd.Flags().Changed(flagPlatform):
		mediaBuildOpts.Platform = downloadCmdFlags.platform
	case spec.Sbc != nil:
		mediaBuildOpts.Overlay = spec.Sbc.Overlay
		mediaBuildOpts.OverlayOptions = spec.Sbc.OverlayOptions

		if hasOverlayOptions {
			mediaBuildOpts.OverlayOptions = downloadCmdFlags.overlayOptions
		}
	case spec.Cloud != nil:
		mediaBuildOpts.Platform = spec.Cloud.Platform
	}

	if hasOverlayOptions && mediaBuildOpts.Overlay == "" {
		fmt.Fprintf(os.Stderr, "WARNING: --%s is ignored; preset %q is not SBC-based and --%s was not specified\n",
			flagOverlayOptions, presetName, flagOverlay)
	}

	// Resolve overlay before building the image so invalid overlays fail fast.
	var overlayParams *management.CreateSchematicRequest_Overlay

	if mediaBuildOpts.Overlay != "" {
		overlayParams, err = download.ResolveOverlay(ctx, client, mediaBuildOpts.Overlay, mediaBuildOpts.OverlayOptions)
		if err != nil {
			return err
		}
	}

	image, err := download.BuildImageFromPreset(ctx, client, presetName, arch, mediaBuildOpts)
	if err != nil {
		return err
	}

	params, err := download.BuildParamsFromPreset(spec, arch)
	if err != nil {
		return err
	}

	params.Output = output
	params.PXE = downloadCmdFlags.format == download.FormatPXE
	params.Overlay = overlayParams

	if cmd.Flags().Changed(flagTalosVersion) {
		params.TalosVersion = downloadCmdFlags.talosVersion
	}

	if cmd.Flags().Changed(flagInitialLabels) {
		params.Labels = downloadCmdFlags.labels
	}

	if cmd.Flags().Changed(flagExtraKernelArgs) {
		params.ExtraKernelArgs = downloadCmdFlags.extraKernelArgs
	}

	if cmd.Flags().Changed(flagExtensions) {
		params.Extensions = downloadCmdFlags.extensions
	}

	if cmd.Flags().Changed(flagSecureBoot) {
		params.SecureBoot = downloadCmdFlags.secureBoot
	}

	if cmd.Flags().Changed(flagBootloader) {
		params.Bootloader = download.ParseBootloader(downloadCmdFlags.bootloader)
	}

	// Empty tokenSource falls back to the server's default token in ResolveJoinToken.
	tokenSource := params.JoinToken
	if cmd.Flags().Changed(flagJoinTokenName) {
		tokenSource = downloadCmdFlags.joinTokenName
	}

	params.JoinToken, err = download.ResolveJoinToken(ctx, client, tokenSource)
	if err != nil {
		return err
	}

	params.GrpcTunnelMode = grpcTunnelModeFromPreset(cmd, spec)

	return download.DownloadImageTo(ctx, client, image, params)
}

func grpcTunnelModeFromPreset(cmd *cobra.Command, spec *specs.InstallationMediaConfigSpec) management.CreateSchematicRequest_SiderolinkGRPCTunnelMode {
	if cmd.Flags().Changed(flagGRPCTunnel) {
		return download.GRPCTunnelModeFromFlag(cmd, flagGRPCTunnel, downloadCmdFlags.useSiderolinkGRPCTunnel)
	}

	switch spec.GrpcTunnel { //nolint:exhaustive
	case specs.GrpcTunnelMode_ENABLED:
		return management.CreateSchematicRequest_ENABLED
	case specs.GrpcTunnelMode_DISABLED:
		return management.CreateSchematicRequest_DISABLED
	default:
		return management.CreateSchematicRequest_AUTO
	}
}

func presetCompletion(toComplete string) ([]string, cobra.ShellCompDirective) {
	var results []string

	err := access.WithClient(
		func(ctx context.Context, client *client.Client, _ access.ServerInfo) error {
			presets, err := safe.StateListAll[*omni.InstallationMediaConfig](ctx, client.Omni().State())
			if err != nil {
				return err
			}

			for preset := range presets.All() {
				name := preset.Metadata().ID()
				if toComplete == "" || strings.Contains(name, toComplete) {
					results = append(results, name)
				}
			}

			return nil
		},
	)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	return results, cobra.ShellCompDirectiveNoFileComp
}
