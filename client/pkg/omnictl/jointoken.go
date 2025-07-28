// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package omnictl

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/client/pkg/omnictl/internal/access"
)

var (
	joinTokenCreateFlags struct {
		role string

		useUserRole bool
		ttl         time.Duration
	}

	joinTokenRenewFlags struct {
		ttl time.Duration
	}

	// joinTokenCmd represents the jointoken command.
	joinTokenCmd = &cobra.Command{
		Use:     "jointoken",
		Aliases: []string{"jt"},
		Short:   "Manage join tokens",
	}

	joinTokenCreateCmd = &cobra.Command{
		Use:     "create <name>",
		Aliases: []string{"c"},
		Short:   "Create a join token",
		Args:    cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			name := args[0]

			return access.WithClient(func(ctx context.Context, client *client.Client) error {
				token, err := client.Management().CreateJoinToken(ctx, name, joinTokenCreateFlags.ttl)
				if err != nil {
					return err
				}

				fmt.Println(token)

				return nil
			})
		},
	}

	joinTokenRevokeCmd = &cobra.Command{
		Use:     "revoke <id>",
		Aliases: []string{"r"},
		Short:   "Revoke a join token",
		Args:    cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			id := args[0]

			return access.WithClient(func(ctx context.Context, client *client.Client) error {
				_, err := safe.StateUpdateWithConflicts(
					ctx,
					client.Omni().State(),
					siderolink.NewJoinToken(resources.DefaultNamespace, id).Metadata(),
					func(res *siderolink.JoinToken) error {
						res.TypedSpec().Value.Revoked = true

						return nil
					},
				)
				if err != nil {
					return err
				}

				fmt.Printf("token %q was revoked\n", id)

				return nil
			})
		},
	}

	joinTokenUnrevokeCmd = &cobra.Command{
		Use:     "unrevoke <id>",
		Aliases: []string{"ur"},
		Short:   "Unrevoke a join token",
		Args:    cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			id := args[0]

			return access.WithClient(func(ctx context.Context, client *client.Client) error {
				_, err := safe.StateUpdateWithConflicts(
					ctx,
					client.Omni().State(),
					siderolink.NewJoinToken(resources.DefaultNamespace, id).Metadata(),
					func(res *siderolink.JoinToken) error {
						res.TypedSpec().Value.Revoked = false

						return nil
					},
				)
				if err != nil {
					return err
				}

				fmt.Printf("token %q was unrevoked\n", id)

				return nil
			})
		},
	}

	joinTokenMakeDefaultCmd = &cobra.Command{
		Use:     "make-default <id>",
		Aliases: []string{"md"},
		Short:   "Make the token default one",
		Args:    cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			id := args[0]

			return access.WithClient(func(ctx context.Context, client *client.Client) error {
				_, err := safe.StateUpdateWithConflicts(
					ctx,
					client.Omni().State(),
					siderolink.NewDefaultJoinToken().Metadata(),
					func(res *siderolink.DefaultJoinToken) error {
						res.TypedSpec().Value.TokenId = id

						return nil
					},
				)
				if err != nil {
					return err
				}

				fmt.Printf("token %q is now default\n", id)

				return nil
			})
		},
	}

	joinTokenRenewCmd = &cobra.Command{
		Use:     "renew <id>",
		Aliases: []string{"r"},
		Short:   "Renew a join token",
		Args:    cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			id := args[0]

			if joinTokenRenewFlags.ttl == 0 {
				return fmt.Errorf("ttl should be greater than 0")
			}

			return access.WithClient(func(ctx context.Context, client *client.Client) error {
				_, err := safe.StateUpdateWithConflicts(
					ctx,
					client.Omni().State(),
					siderolink.NewJoinToken(resources.DefaultNamespace, id).Metadata(),
					func(res *siderolink.JoinToken) error {
						res.TypedSpec().Value.ExpirationTime = timestamppb.New(time.Now().Add(joinTokenRenewFlags.ttl))

						return nil
					},
				)
				if err != nil {
					return err
				}

				fmt.Printf("token %q was renewed, new ttl is %s\n", id, joinTokenRenewFlags.ttl)

				return nil
			})
		},
	}

	joinTokenListCmd = &cobra.Command{
		Use:     "list",
		Aliases: []string{"l"},
		Short:   "List join tokens",
		Args:    cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			return access.WithClient(func(ctx context.Context, client *client.Client) error {
				joinTokens, err := safe.ReaderListAll[*siderolink.JoinTokenStatus](ctx, client.Omni().State())
				if err != nil {
					return err
				}

				writer := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)

				fmt.Fprintf(writer, "ID\tNAME\tSTATE\tEXPIRATION\tUSE COUNT\tDEFAULT\n") //nolint:errcheck

				for token := range joinTokens.All() {
					var isDefault string

					if token.TypedSpec().Value.IsDefault {
						isDefault = "*"
					}

					expirationTime := "never"

					if token.TypedSpec().Value.ExpirationTime != nil {
						expirationTime = token.TypedSpec().Value.ExpirationTime.AsTime().String()
					}

					if _, err = fmt.Fprintf(
						writer,
						"%s\t%s\t%s\t%s\t%d\t%s\n",
						token.Metadata().ID(),
						token.TypedSpec().Value.Name,
						token.TypedSpec().Value.State.String(),
						expirationTime,
						token.TypedSpec().Value.UseCount,
						isDefault,
					); err != nil {
						return err
					}
				}

				return writer.Flush()
			})
		},
	}

	joinTokenDeleteCmd = &cobra.Command{
		Use:     "delete <name>",
		Aliases: []string{"d"},
		Short:   "Delete a join token",
		Args:    cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			id := args[0]

			return access.WithClient(func(ctx context.Context, client *client.Client) error {
				err := client.Omni().State().TeardownAndDestroy(ctx, siderolink.NewJoinToken(resources.DefaultNamespace, id).Metadata())
				if err != nil {
					return fmt.Errorf("failed to delete a join token: %w", err)
				}

				fmt.Printf("deleted join token: %s\n", id)

				return nil
			})
		},
	}
)

func init() {
	RootCmd.AddCommand(joinTokenCmd)

	joinTokenCmd.AddCommand(joinTokenCreateCmd)
	joinTokenCmd.AddCommand(joinTokenListCmd)
	joinTokenCmd.AddCommand(joinTokenDeleteCmd)
	joinTokenCmd.AddCommand(joinTokenRevokeCmd)
	joinTokenCmd.AddCommand(joinTokenMakeDefaultCmd)
	joinTokenCmd.AddCommand(joinTokenUnrevokeCmd)
	joinTokenCmd.AddCommand(joinTokenRenewCmd)

	joinTokenCreateCmd.Flags().DurationVarP(&joinTokenCreateFlags.ttl, "ttl", "t", 0, "TTL for the join token")

	joinTokenRenewCmd.Flags().DurationVarP(&joinTokenRenewFlags.ttl, "ttl", "t", 0, "TTL for the join token")

	joinTokenRenewCmd.MarkFlagRequired("ttl") //nolint:errcheck
}
