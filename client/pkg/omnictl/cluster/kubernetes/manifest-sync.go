// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubernetes

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/siderolabs/omni/client/api/omni/management"
	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/omnictl/internal/access"
)

var manifestSyncCmdFlags struct {
	dryRun bool
}

// manifestSyncCmd represents the cluster kubernetes manifest-sync command.
var manifestSyncCmd = &cobra.Command{
	Use:   "manifest-sync cluster-name",
	Short: "Sync Kubernetes bootstrap manifests from Talos controlplane nodes to Kubernetes API.",
	Long: `Sync Kubernetes bootstrap manifests from Talos controlplane nodes to Kubernetes API.
Bootstrap manifests might be updated with Talos version update, Kubernetes upgrade, and config patching.
Talos never updates or deletes Kubernetes manifests, so this command fills the gap to keep manifests up-to-date.`,
	Example: "",
	Args:    cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		return access.WithClient(manifestSync(args[0]))
	},
}

func manifestSync(clusterName string) func(ctx context.Context, client *client.Client) error {
	return func(ctx context.Context, client *client.Client) error {
		handler := func(resp *management.KubernetesSyncManifestResponse) error {
			switch resp.ResponseType {
			case management.KubernetesSyncManifestResponse_UNKNOWN:
			case management.KubernetesSyncManifestResponse_MANIFEST:
				fmt.Printf(" > processing manifest %s\n", resp.Path)

				switch {
				case resp.Skipped:
					fmt.Println(" < no changes")
				case manifestSyncCmdFlags.dryRun:
					fmt.Println(resp.Diff)
					fmt.Println(" < dry run, change skipped")
				case !manifestSyncCmdFlags.dryRun:
					fmt.Println(resp.Diff)
					fmt.Println(" < applied successfully")
				}
			case management.KubernetesSyncManifestResponse_ROLLOUT:
				fmt.Printf(" > waiting for %s\n", resp.Path)
			}

			return nil
		}

		return client.Management().WithCluster(clusterName).KubernetesSyncManifests(ctx, manifestSyncCmdFlags.dryRun, handler)
	}
}

func init() {
	manifestSyncCmd.Flags().BoolVar(&manifestSyncCmdFlags.dryRun, "dry-run", true, "don't actually sync manifests, just print what would be done")
	kubernetesCmd.AddCommand(manifestSyncCmd)
}
