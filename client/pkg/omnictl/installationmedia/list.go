// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package installationmedia

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/cosi-project/runtime/pkg/safe"
	xmaps "github.com/siderolabs/gen/maps"
	"github.com/spf13/cobra"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omnictl/internal/access"
	"github.com/siderolabs/omni/client/pkg/omnictl/internal/download"
)

var listCmdFlags struct {
	output string
}

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List installation media presets.",
	Args:    cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		if listCmdFlags.output != "" && listCmdFlags.output != "wide" {
			return fmt.Errorf("invalid --output value %q, only \"wide\" is supported", listCmdFlags.output)
		}

		return access.WithClient(func(ctx context.Context, client *client.Client, info access.ServerInfo) error {
			if !info.ServerSupports(1, 7) {
				return fmt.Errorf("installation media presets require Omni v1.7.0 or newer (server is %s)", info.Version)
			}

			return listPresets(ctx, client, listCmdFlags.output == "wide")
		})
	},
}

func init() {
	listCmd.Flags().StringVarP(&listCmdFlags.output, "output", "o", "", "Output format. One of: wide")

	presetCmd.AddCommand(listCmd)
}

func listPresets(ctx context.Context, client *client.Client, wide bool) error {
	presets, err := safe.StateListAll[*omni.InstallationMediaConfig](ctx, client.Omni().State())
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)

	header := "NAME\tARCH\tTALOS VERSION\tTYPE\tPLATFORM/OVERLAY"
	if wide {
		header += "\tSECURE BOOT\tEXTENSIONS\tLABELS\tBOOTLOADER\tGRPC TUNNEL\tKERNEL ARGS"
	}

	fmt.Fprintln(w, header) //nolint:errcheck

	for preset := range presets.All() {
		spec := preset.TypedSpec().Value

		talosVersion := spec.TalosVersion
		if spec.TalosVersion == "" {
			talosVersion = constants.DefaultTalosVersion
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s", //nolint:errcheck
			preset.Metadata().ID(),
			download.ArchToString(spec.Architecture),
			talosVersion,
			presetType(spec),
			platformOrOverlay(spec),
		)

		if wide {
			fmt.Fprintf(w, "\t%v\t%s\t%s\t%s\t%s\t%s", //nolint:errcheck
				spec.SecureBoot,
				extensionsOrDash(spec.InstallExtensions),
				machineLabelsOrDash(spec.MachineLabels),
				download.BootloaderToString(spec.Bootloader),
				download.GrpcTunnelModeToString(spec.GrpcTunnel),
				kernelArgsOrDash(spec.KernelArgs),
			)
		}

		fmt.Fprintln(w) //nolint:errcheck
	}

	return w.Flush()
}

func presetType(spec *specs.InstallationMediaConfigSpec) string {
	switch {
	case spec.Cloud != nil:
		return "cloud"
	case spec.Sbc != nil:
		return "sbc"
	default:
		return "metal"
	}
}

func platformOrOverlay(spec *specs.InstallationMediaConfigSpec) string {
	switch {
	case spec.Cloud != nil:
		return spec.Cloud.Platform
	case spec.Sbc != nil:
		return spec.Sbc.Overlay
	default:
		return "-"
	}
}

func kernelArgsOrDash(args string) string {
	if args == "" {
		return "-"
	}

	return args
}

func extensionsOrDash(extensions []string) string {
	if len(extensions) == 0 {
		return "-"
	}

	return strings.Join(extensions, " ")
}

func machineLabelsOrDash(labels map[string]string) string {
	if len(labels) == 0 {
		return "-"
	}

	return strings.Join(xmaps.ToSlice(labels, func(k, v string) string {
		if v == "" {
			return k
		}

		return k + "=" + v
	}), " ")
}
