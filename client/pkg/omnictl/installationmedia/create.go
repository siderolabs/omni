// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package installationmedia

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omnictl/internal/access"
	"github.com/siderolabs/omni/client/pkg/omnictl/internal/download"
)

var createCmdFlags struct {
	arch            string
	talosVersion    string
	platform        string
	overlay         string
	overlayOptions  string
	joinTokenName   string
	bootloader      string
	extensions      []string
	extraKernelArgs []string
	labels          []string
	secureBoot      bool
	grpcTunnel      bool
}

var createCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create an installation media preset.",
	Long: `Create a new installation media preset that can be used for downloading installation media.

Examples:
    # Create a basic metal preset
    omnictl media preset create my-preset --arch amd64

    # Create a cloud preset for AWS
    omnictl media preset create aws-preset --arch amd64 --platform aws

    # Create an SBC preset for Raspberry Pi
    omnictl media preset create rpi-preset --arch arm64 --overlay rpi_generic

    # Create a preset with extensions and labels
    omnictl media preset create full-preset --arch amd64 \
        --extensions qemu-guest-agent --extensions intel-ucode \
        --initial-labels env=production --initial-labels team=infra
`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := validateCreateFlags(cmd); err != nil {
			return err
		}

		return access.WithClient(func(ctx context.Context, client *client.Client, info access.ServerInfo) error {
			if !info.ServerSupports(1, 7) {
				return fmt.Errorf("installation media presets require Omni v1.7.0 or newer (server is %s)", info.Version)
			}

			return createPreset(ctx, cmd, client, args[0])
		})
	},
}

func init() {
	createCmd.Flags().StringVar(&createCmdFlags.arch, flagArch, "amd64", "Image architecture (amd64, arm64)")
	createCmd.Flags().StringVar(&createCmdFlags.talosVersion, flagTalosVersion, "", "Talos version for the preset (leave empty to track the server's default at download time)")
	createCmd.Flags().StringSliceVar(&createCmdFlags.extensions, flagExtensions, nil, "Extensions to pre-install in the preset")
	createCmd.Flags().StringArrayVar(&createCmdFlags.extraKernelArgs, flagExtraKernelArgs, nil, "Extra kernel args to include in the preset")
	createCmd.Flags().StringSliceVar(&createCmdFlags.labels, flagInitialLabels, nil, "Initial machine labels in key=value format")
	createCmd.Flags().StringVar(&createCmdFlags.platform, flagPlatform, "", "Cloud platform (e.g., aws, gcp, azure, vultr)")
	createCmd.Flags().StringVar(&createCmdFlags.overlay, flagOverlay, "", "SBC overlay name (e.g., rpi_generic, rockpi_4c)")
	createCmd.Flags().StringVar(&createCmdFlags.overlayOptions, flagOverlayOptions, "", "SBC overlay options (YAML string)")
	createCmd.Flags().BoolVar(&createCmdFlags.secureBoot, flagSecureBoot, false, "Enable SecureBoot in the preset")
	createCmd.Flags().StringVar(&createCmdFlags.joinTokenName, flagJoinTokenName, "", "Join token ID (uses default token if empty)")
	createCmd.Flags().StringVar(&createCmdFlags.bootloader, flagBootloader, "auto", "Bootloader type (auto, uefi, bios, dual)")
	createCmd.Flags().BoolVar(&createCmdFlags.grpcTunnel, flagGRPCTunnel, false,
		"Configure Talos to use the SideroLink (WireGuard) gRPC tunnel over HTTP/2 for Omni management traffic, instead of UDP."+
			" Only enable this if the network blocks UDP packets, as HTTP tunneling adds significant overhead for communications.")

	presetCmd.AddCommand(createCmd)
}

func validateCreateFlags(cmd *cobra.Command) error {
	if cmd.Flags().Changed(flagPlatform) && cmd.Flags().Changed(flagOverlay) {
		return fmt.Errorf("--%s and --%s cannot be used together; a preset can be for a cloud platform, an SBC overlay or Bare-metal", flagPlatform, flagOverlay)
	}

	if cmd.Flags().Changed(flagOverlayOptions) && !cmd.Flags().Changed(flagOverlay) {
		return fmt.Errorf("--%s requires --%s to be set", flagOverlayOptions, flagOverlay)
	}

	return nil
}

func createPreset(ctx context.Context, cmd *cobra.Command, client *client.Client, name string) error {
	arch, err := download.ParseArch(createCmdFlags.arch)
	if err != nil {
		return err
	}

	// If the user explicitly passed --join-token, validate it now and store it. Otherwise leave it
	// empty so the preset tracks whichever join token is the server's default at download time.
	var tokenID string

	if cmd.Flags().Changed(flagJoinTokenName) {
		tokenID, err = download.ResolveJoinToken(ctx, client, createCmdFlags.joinTokenName)
		if err != nil {
			return err
		}
	}

	// An empty user value means "use the server's default at download time"; for create-time
	// validation against platform min-versions and extension catalogs, fall back to the CLI's
	// default Talos version so the checks still run.
	validationTalosVersion := createCmdFlags.talosVersion
	if validationTalosVersion == "" {
		validationTalosVersion = constants.DefaultTalosVersion
	}

	if err = download.ValidateTalosVersion(ctx, client.Omni().State(), validationTalosVersion); err != nil {
		return err
	}

	bootloader, err := download.ParseBootloader(createCmdFlags.bootloader)
	if err != nil {
		return err
	}

	spec := &specs.InstallationMediaConfigSpec{
		TalosVersion:      createCmdFlags.talosVersion,
		Architecture:      arch,
		InstallExtensions: createCmdFlags.extensions,
		KernelArgs:        strings.Join(createCmdFlags.extraKernelArgs, " "),
		JoinToken:         tokenID,
		SecureBoot:        createCmdFlags.secureBoot,
		Bootloader:        bootloader,
	}

	switch {
	case !cmd.Flags().Changed(flagGRPCTunnel):
		spec.GrpcTunnel = specs.GrpcTunnelMode_UNSET
	case createCmdFlags.grpcTunnel:
		spec.GrpcTunnel = specs.GrpcTunnelMode_ENABLED
	default:
		spec.GrpcTunnel = specs.GrpcTunnelMode_DISABLED
	}

	if createCmdFlags.platform != "" {
		if err = download.ValidateCloudPlatform(ctx, client.Omni().State(), createCmdFlags.platform, arch, createCmdFlags.secureBoot, validationTalosVersion); err != nil {
			return err
		}

		spec.Cloud = &specs.InstallationMediaConfigSpec_Cloud{
			Platform: createCmdFlags.platform,
		}
	}

	if createCmdFlags.overlay != "" {
		if err = download.ValidateSBC(ctx, client.Omni().State(), createCmdFlags.overlay, validationTalosVersion); err != nil {
			return err
		}

		spec.Sbc = &specs.InstallationMediaConfigSpec_SBC{
			Overlay:        createCmdFlags.overlay,
			OverlayOptions: createCmdFlags.overlayOptions,
		}
	}

	if err = download.ValidateExtensions(ctx, client.Omni().State(), validationTalosVersion, createCmdFlags.extensions); err != nil {
		return err
	}

	if createCmdFlags.labels != nil {
		spec.MachineLabels, err = download.ParseLabelPairs(createCmdFlags.labels)
		if err != nil {
			return err
		}
	}

	res := omni.NewInstallationMediaConfig(name)
	res.TypedSpec().Value = spec

	if err = client.Omni().State().Create(ctx, res); err != nil {
		return fmt.Errorf("failed to create installation media preset: %w", err)
	}

	fmt.Printf("Created installation media preset %q\n", name)

	return nil
}
