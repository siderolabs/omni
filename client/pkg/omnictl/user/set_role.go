// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package user

import (
	"context"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/spf13/cobra"

	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/client/pkg/omnictl/internal/access"
)

var setRoleCmdFlags struct {
	role string
}

// setRoleCmd represents the user role set command.
var setRoleCmd = &cobra.Command{
	Use:     "set-role [email]",
	Short:   "Update the role of the user.",
	Long:    `Update the user role.`,
	Example: "",
	Args:    cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		return access.WithClient(setUserRole(args[0]))
	},
}

func setUserRole(email string) func(ctx context.Context, client *client.Client) error {
	return func(ctx context.Context, client *client.Client) error {
		identity, err := safe.ReaderGetByID[*auth.Identity](ctx, client.Omni().State(), email)
		if err != nil {
			return err
		}

		_, err = safe.StateUpdateWithConflicts(ctx, client.Omni().State(),
			auth.NewUser(resources.DefaultNamespace, identity.TypedSpec().Value.UserId).Metadata(),
			func(user *auth.User) error {
				user.TypedSpec().Value.Role = setRoleCmdFlags.role

				return nil
			},
		)
		if err != nil {
			return err
		}

		return nil
	}
}

func init() {
	setRoleCmd.PersistentFlags().StringVarP(&setRoleCmdFlags.role, "role", "r", "", "Role to use")
	setRoleCmd.MarkPersistentFlagRequired("role") //nolint:errcheck

	userCmd.AddCommand(setRoleCmd)
}
