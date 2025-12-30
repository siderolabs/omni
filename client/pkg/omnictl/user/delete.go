// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package user

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/spf13/cobra"

	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/client/pkg/omnictl/internal/access"
)

// deleteCmd represents the user delete command.
var deleteCmd = &cobra.Command{
	Use:     "delete [email1 email2]",
	Short:   "Delete users.",
	Long:    `Delete users with the specified emails.`,
	Example: "",
	Args:    cobra.MinimumNArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		return access.WithClient(deleteUsers(args...))
	},
}

func deleteUsers(emails ...string) func(ctx context.Context, client *client.Client) error {
	return func(ctx context.Context, client *client.Client) error {
		toDelete := make([]resource.Pointer, 0, len(emails)*2)

		for _, email := range emails {
			identity := auth.NewIdentity(email)

			existing, err := safe.ReaderGetByID[*auth.Identity](ctx, client.Omni().State(), email)
			if err != nil {
				return err
			}

			toDelete = append(toDelete, identity.Metadata(), auth.NewUser(existing.TypedSpec().Value.UserId).Metadata())
		}

		for _, md := range toDelete {
			fmt.Printf("tearing down %s %s\n", md.Type(), md.ID())

			if _, err := client.Omni().State().Teardown(ctx, md); err != nil {
				return err
			}
		}

		for _, md := range toDelete {
			_, err := client.Omni().State().WatchFor(ctx, md, state.WithFinalizerEmpty())
			if err != nil {
				return err
			}

			err = client.Omni().State().Destroy(ctx, md)
			if err != nil {
				return err
			}

			fmt.Printf("destroy %s %s\n", md.Type(), md.ID())
		}

		return nil
	}
}

func init() {
	userCmd.AddCommand(deleteCmd)
}
