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

	"github.com/spf13/cobra"

	"github.com/siderolabs/omni/client/pkg/client"
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
	users, err := client.Management().ListUsers(ctx)
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	defer w.Flush() //nolint:errcheck

	if _, err = fmt.Fprintln(w, "ID\tEMAIL\tROLE\tLAST ACTIVE\tLABELS"); err != nil {
		return err
	}

	for _, user := range users {
		labels := formatSAMLLabels(user.SamlLabels)

		lastActive := user.LastActive
		if lastActive == "" {
			lastActive = "Never"
		}

		if _, err = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", user.Id, user.Email, user.Role, lastActive, labels); err != nil {
			return err
		}
	}

	return nil
}

func formatSAMLLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}

	result := make([]string, 0, len(labels))

	for key, value := range labels {
		result = append(result, key+"="+value)
	}

	slices.Sort(result)

	return strings.Join(result, ", ")
}

func init() {
	userCmd.AddCommand(listCmd)
}
