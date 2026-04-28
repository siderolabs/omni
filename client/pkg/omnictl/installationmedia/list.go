// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package installationmedia

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/spf13/cobra"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omnictl/internal/access"
	"github.com/siderolabs/omni/client/pkg/omnictl/internal/download"
)

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List installation media presets.",
	Args:    cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		return access.WithClient(func(ctx context.Context, client *client.Client, info access.ServerInfo) error {
			if !info.ServerSupports(1, 7) {
				return fmt.Errorf("installation media presets require Omni v1.7.0 or newer (server is %s)", info.Version)
			}

			return listPresets(ctx, client)
		})
	},
}

func init() {
	cmd.AddCommand(listCmd)
}

func listPresets(ctx context.Context, client *client.Client) error {
	presets, err := safe.StateListAll[*omni.InstallationMediaConfig](ctx, client.Omni().State())
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)

	fmt.Fprintln(w, "NAME\tARCH\tTALOS VERSION\tTYPE\tSECURE BOOT") //nolint:errcheck

	for preset := range presets.All() {
		spec := preset.TypedSpec().Value

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%v\n", //nolint:errcheck
			preset.Metadata().ID(),
			download.ArchToString(spec.Architecture),
			spec.TalosVersion,
			presetType(spec),
			spec.SecureBoot,
		)
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
