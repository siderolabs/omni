// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package omnictl

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/spf13/cobra"

	"github.com/siderolabs/omni/client/pkg/client"
	omniresources "github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omnictl/internal/access"
	"github.com/siderolabs/omni/client/pkg/omnictl/resources"
)

var deleteCmdFlags struct {
	namespace string
	selector  string
	all       bool
}

// deleteCmd represents the delete (resources) command.
var deleteCmd = &cobra.Command{
	Use:     "delete <type> [<id>]",
	Aliases: []string{"d"},
	Short:   "Delete a specific resource by ID or all resources of the type.",
	Long:    `Similar to 'kubectl delete', 'omnictl delete' initiates resource deletion and waits for the operation to complete.`,
	Example: "",
	Args:    cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return access.WithClient(deleteResources(cmd, args))
	},
}

func deleteResources(cmd *cobra.Command, args []string) func(ctx context.Context, client *client.Client) error {
	return func(ctx context.Context, client *client.Client) error {
		ns := ""

		if cmd.Flags().Lookup("namespace").Changed {
			ns = deleteCmdFlags.namespace
		}

		var ids []resource.ID

		if len(args) > 1 {
			ids = args[1:]
		} else if !deleteCmdFlags.all && deleteCmdFlags.selector == "" {
			return fmt.Errorf("either resource ID or one of --all or --selector flags must be specified")
		}

		return resources.Destroy(ctx, client.Omni().State(), ns, args[0], deleteCmdFlags.selector, deleteCmdFlags.all, ids)
	}
}

func init() {
	deleteCmd.PersistentFlags().StringVarP(&deleteCmdFlags.namespace, "namespace", "n", omniresources.DefaultNamespace, "The resource namespace.")
	deleteCmd.PersistentFlags().BoolVar(&deleteCmdFlags.all, "all", false, "Delete all resources of the type.")
	deleteCmd.PersistentFlags().StringVarP(&deleteCmdFlags.selector, "selector", "l", "", "Selector (label query) to filter on, supports '=' and '==' (e.g. -l key1=value1,key2=value2)")

	deleteCmd.MarkFlagsMutuallyExclusive("all", "selector")

	RootCmd.AddCommand(deleteCmd)
}
