// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package user

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"
	"text/tabwriter"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/spf13/cobra"

	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/client/pkg/omnictl/internal/access"
)

// listCmd represents the user list command.
var listCmd = &cobra.Command{
	Use:     "list",
	Short:   "List all users.",
	Long:    `List all existing users on the Omni instance.`,
	Example: "",
	Args:    cobra.ExactArgs(0),
	RunE: func(*cobra.Command, []string) error {
		return access.WithClient(listUsers)
	},
}

func listUsers(ctx context.Context, client *client.Client) error {
	identities, err := safe.ReaderListAll[*auth.Identity](ctx, client.Omni().State(), state.WithLabelQuery(
		resource.LabelExists(auth.LabelIdentityTypeServiceAccount, resource.NotMatches),
	))
	if err != nil {
		return err
	}

	type user struct {
		id     string
		email  string
		role   string
		labels string
	}

	users, err := safe.ReaderListAll[*auth.User](ctx, client.Omni().State())
	if err != nil {
		return err
	}

	userList := safe.ToSlice(identities, func(identity *auth.Identity) user {
		res := user{
			id:    identity.TypedSpec().Value.UserId,
			email: identity.Metadata().ID(),
		}

		u, found := users.Find(func(user *auth.User) bool {
			return user.Metadata().ID() == identity.TypedSpec().Value.UserId
		})
		if !found {
			return res
		}

		res.role = u.TypedSpec().Value.Role

		allLabels := identity.Metadata().Labels().Raw()

		samlLabels := make([]string, 0, len(allLabels))

		for key, value := range allLabels {
			if !strings.HasPrefix(key, auth.SAMLLabelPrefix) {
				continue
			}

			samlLabels = append(samlLabels, strings.TrimPrefix(key, auth.SAMLLabelPrefix)+"="+value)
		}

		slices.Sort(samlLabels)

		res.labels = strings.Join(samlLabels, ", ")

		return res
	})

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	defer w.Flush() //nolint:errcheck

	_, err = fmt.Fprintln(w, "ID\tEMAIL\tROLE\tLABELS")
	if err != nil {
		return err
	}

	for _, user := range userList {
		_, err = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", user.id, user.email, user.role, user.labels)
		if err != nil {
			return err
		}
	}

	return nil
}

func init() {
	userCmd.AddCommand(listCmd)
}
