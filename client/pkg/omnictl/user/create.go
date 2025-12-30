// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package user

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/client/pkg/omnictl/internal/access"
)

var createCmdFlags struct {
	role string
}

// createCmd represents the user create command.
var createCmd = &cobra.Command{
	Use:     "create [email]",
	Short:   "Create a user.",
	Long:    `Create a user with the specified email.`,
	Example: "",
	Args:    cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		return access.WithClient(createUser(args[0]))
	},
}

func createUser(email string) func(ctx context.Context, client *client.Client) error {
	return func(ctx context.Context, client *client.Client) error {
		user := auth.NewUser(uuid.NewString())

		user.TypedSpec().Value.Role = createCmdFlags.role

		identity := auth.NewIdentity(email)

		identity.Metadata().Labels().Set(auth.LabelIdentityUserID, user.Metadata().ID())

		identity.TypedSpec().Value.UserId = user.Metadata().ID()

		existing, err := safe.ReaderGetByID[*auth.Identity](ctx, client.Omni().State(), email)
		if err != nil && !state.IsNotFoundError(err) {
			return err
		}

		if existing != nil {
			return fmt.Errorf("identity with email %q already exists", email)
		}

		if err := client.Omni().State().Create(ctx, user); err != nil {
			return err
		}

		return client.Omni().State().Create(ctx, identity)
	}
}

func init() {
	createCmd.PersistentFlags().StringVarP(&createCmdFlags.role, "role", "r", "", "Role to use for the user creation")
	createCmd.MarkPersistentFlagRequired("role") //nolint:errcheck

	userCmd.AddCommand(createCmd)
}
