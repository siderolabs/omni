// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubernetes

import (
	"context"

	"github.com/siderolabs/gen/ensure"
	"github.com/spf13/cobra"

	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/omnictl/internal/access"
)

var upgradePreChecksCmdFlags struct {
	toVersion string
}

// upgradePreChecksCmd represents the cluster kubernetes upgrade-pre-checks command.
var upgradePreChecksCmd = &cobra.Command{
	Use:     "upgrade-pre-checks cluster-name",
	Short:   "Run Kubernetes upgrade pre-checks for the cluster.",
	Long:    `Verify that upgrading Kubernetes version is available for the cluster: version compatibility, deprecated APIs, etc.`,
	Example: "",
	Args:    cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		return access.WithClient(upgradePreChecks(args[0]))
	},
}

func upgradePreChecks(clusterName string) func(ctx context.Context, client *client.Client) error {
	return func(ctx context.Context, client *client.Client) error {
		return client.Management().WithCluster(clusterName).KubernetesUpgradePreChecks(ctx, upgradePreChecksCmdFlags.toVersion)
	}
}

func init() {
	upgradePreChecksCmd.Flags().StringVar(&upgradePreChecksCmdFlags.toVersion, "to", "", "target Kubernetes version for the planned upgrade")
	ensure.NoError(upgradePreChecksCmd.MarkFlagRequired("to"))
	kubernetesCmd.AddCommand(upgradePreChecksCmd)
}
