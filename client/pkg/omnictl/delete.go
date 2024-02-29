// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package omnictl

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/spf13/cobra"

	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/cosi/labels"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omnictl/internal/access"
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
	Args:    cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return access.WithClient(deleteResources(cmd, args))
	},
}

//nolint:gocognit,gocyclo,cyclop
func deleteResources(cmd *cobra.Command, args []string) func(ctx context.Context, client *client.Client) error {
	return func(ctx context.Context, client *client.Client) error {
		st := client.Omni().State()

		resourceType := resource.Type(args[0]) //nolint:unconvert

		rd, err := resolveResourceType(ctx, st, resourceType)
		if err != nil {
			return err
		}

		if !cmd.Flags().Lookup("namespace").Changed {
			deleteCmdFlags.namespace = rd.TypedSpec().DefaultNamespace
		}

		var resourceIDs []resource.ID

		if len(args) > 1 {
			resourceIDs = []resource.ID{args[1]}
		} else {
			var listOpts []state.ListOption

			if !deleteCmdFlags.all && deleteCmdFlags.selector == "" {
				return fmt.Errorf("either resource ID or one of --all or --selector flags must be specified")
			}

			if deleteCmdFlags.selector != "" {
				var query *resource.LabelQuery

				query, err = labels.ParseQuery(deleteCmdFlags.selector)
				if err != nil {
					return err
				}

				listOpts = append(listOpts, state.WithLabelQuery(resource.RawLabelQuery(*query)))
			}

			var list resource.List

			list, err = st.List(ctx, resource.NewMetadata(deleteCmdFlags.namespace, rd.TypedSpec().Type, "", resource.VersionUndefined), listOpts...)
			if err != nil {
				return err
			}

			resourceIDs = make([]resource.ID, 0, len(list.Items))

			for _, item := range list.Items {
				resourceIDs = append(resourceIDs, item.Metadata().ID())
			}
		}

		// teardown all resources
		for _, resourceID := range resourceIDs {
			_, err = st.Teardown(ctx, resource.NewMetadata(deleteCmdFlags.namespace, rd.TypedSpec().Type, resourceID, resource.VersionUndefined))
			if err != nil {
				return err
			}

			fmt.Printf("torn down %s %s\n", rd.TypedSpec().Type, resourceID)
		}

		// set up a watch for all resources of kind
		watchCh := make(chan state.Event)

		for _, resourceID := range resourceIDs {
			err = st.Watch(ctx, resource.NewMetadata(deleteCmdFlags.namespace, rd.TypedSpec().Type, resourceID, resource.VersionUndefined), watchCh)
			if err != nil {
				return err
			}
		}

		resourceIDsLeft := map[resource.ID]struct{}{}

		for _, resourceID := range resourceIDs {
			resourceIDsLeft[resourceID] = struct{}{}
		}

		// until some resources are not deleted yet...
		for len(resourceIDsLeft) > 0 {
			var event state.Event

			select {
			case <-ctx.Done():
				return ctx.Err()
			case event = <-watchCh:
			}

			switch event.Type {
			case state.Destroyed:
				delete(resourceIDsLeft, event.Resource.Metadata().ID())
			case state.Created, state.Updated:
				if _, ours := resourceIDsLeft[event.Resource.Metadata().ID()]; !ours {
					continue
				}

				if event.Resource.Metadata().Phase() == resource.PhaseTearingDown && event.Resource.Metadata().Finalizers().Empty() {
					if err = st.Destroy(ctx, event.Resource.Metadata()); err != nil && !state.IsNotFoundError(err) {
						return err
					}

					fmt.Printf("destroyed %s %s\n", rd.TypedSpec().Type, event.Resource.Metadata().ID())
				}
			case state.Bootstrapped:
				// ignore
			case state.Errored:
				return fmt.Errorf("error watching for resource deletion: %w", event.Error)
			}
		}

		return nil
	}
}

func init() {
	deleteCmd.PersistentFlags().StringVarP(&deleteCmdFlags.namespace, "namespace", "n", resources.DefaultNamespace, "The resource namespace.")
	deleteCmd.PersistentFlags().BoolVar(&deleteCmdFlags.all, "all", false, "Delete all resources of the type.")
	deleteCmd.PersistentFlags().StringVarP(&deleteCmdFlags.selector, "selector", "l", "", "Selector (label query) to filter on, supports '=' and '==' (e.g. -l key1=value1,key2=value2)")

	deleteCmd.MarkFlagsMutuallyExclusive("all", "selector")

	RootCmd.AddCommand(deleteCmd)
}
