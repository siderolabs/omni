// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package installationmedia

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/spf13/cobra"

	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omnictl/internal/access"
)

var deleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete an installation media preset.",
	Args:  cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		return access.WithClient(func(ctx context.Context, client *client.Client, info access.ServerInfo) error {
			if !info.ServerSupports(1, 7) {
				return fmt.Errorf("installation media presets require Omni v1.7.0 or newer (server is %s)", info.Version)
			}

			return deletePreset(ctx, client, args[0])
		})
	},
}

func init() {
	cmd.AddCommand(deleteCmd)
}

func deletePreset(ctx context.Context, client *client.Client, name string) error {
	md := resource.NewMetadata(resources.DefaultNamespace, omni.InstallationMediaConfigType, name, resource.VersionUndefined)

	if err := client.Omni().State().TeardownAndDestroy(ctx, md); err != nil {
		return fmt.Errorf("failed to delete installation media preset %q: %w", name, err)
	}

	fmt.Printf("Deleted installation media preset %q\n", name)

	return nil
}
