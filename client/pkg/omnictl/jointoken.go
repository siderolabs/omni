// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package omnictl

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/fatih/color"
	"github.com/gertd/go-pluralize"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/siderolabs/omni/client/pkg/client"
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

	joinTokenMachineJoinConfigFlags struct {
		role          string
		tokenID       string
		tokenName     string
		useGRPCTunnel bool
	}

	joinTokenRevokeFlags struct {
		force bool
	}

	joinTokenDeleteFlags struct {
		force bool
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
				if err := checkTokenWarnings(ctx, client, id, "revoke"); err != nil {
					return err
				}

				_, err := safe.StateUpdateWithConflicts(
					ctx,
					client.Omni().State(),
					siderolink.NewJoinToken(id).Metadata(),
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
					siderolink.NewJoinToken(id).Metadata(),
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
					siderolink.NewJoinToken(id).Metadata(),
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

	joinTokenMachineConfigCmd = &cobra.Command{
		Use:     "machine-config <id>",
		Aliases: []string{"mc"},
		Short:   "Get partial machine config to make a machine join Omni",
		Args:    cobra.NoArgs,
		RunE: func(_ *cobra.Command, args []string) error {
			return access.WithClient(func(ctx context.Context, client *client.Client) error {
				tokenID, err := getJoinToken(ctx, client)
				if err != nil {
					return err
				}

				resp, err := client.Management().GetMachineJoinConfig(ctx, tokenID, joinTokenMachineJoinConfigFlags.useGRPCTunnel)
				if err != nil {
					return err
				}

				fmt.Println(resp.Config)

				return nil
			})
		},
	}

	joinTokenKernelArgsCmd = &cobra.Command{
		Use:     "kernel-args",
		Aliases: []string{"ka"},
		Short:   "Get Talos kernel args to make a machine join Omni",
		Args:    cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			return access.WithClient(func(ctx context.Context, client *client.Client) error {
				tokenID, err := getJoinToken(ctx, client)
				if err != nil {
					return err
				}

				resp, err := client.Management().GetMachineJoinConfig(ctx, tokenID, joinTokenMachineJoinConfigFlags.useGRPCTunnel)
				if err != nil {
					return err
				}

				fmt.Println(strings.Join(resp.KernelArgs, " "))

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
				if err := checkTokenWarnings(ctx, client, id, "delete"); err != nil {
					return err
				}

				err := client.Omni().State().TeardownAndDestroy(ctx, siderolink.NewJoinToken(id).Metadata())
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
	joinTokenCmd.AddCommand(joinTokenMachineConfigCmd)
	joinTokenCmd.AddCommand(joinTokenKernelArgsCmd)

	joinTokenRevokeCmd.Flags().BoolVarP(&joinTokenRevokeFlags.force, "force", "f", false, "Revoke the token even if it is going to make the machines to disconnect")

	joinTokenDeleteCmd.Flags().BoolVarP(&joinTokenDeleteFlags.force, "force", "f", false, "Delete the token even if it is going to make the machines to disconnect")

	joinTokenCreateCmd.Flags().DurationVarP(&joinTokenCreateFlags.ttl, "ttl", "t", 0, "TTL for the join token")

	joinTokenRenewCmd.Flags().DurationVarP(&joinTokenRenewFlags.ttl, "ttl", "t", 0, "TTL for the join token")

	addConfigFlags := func(c *cobra.Command) {
		c.Flags().BoolVar(
			&joinTokenMachineJoinConfigFlags.useGRPCTunnel,
			"use-grpc-tunnel", false, "Use gRPC tunnel in the config",
		)

		c.Flags().StringVar(
			&joinTokenMachineJoinConfigFlags.tokenID,
			"token-id", "", "Generate using specific token ID (uses default if empty)",
		)

		c.Flags().StringVar(
			&joinTokenMachineJoinConfigFlags.tokenName,
			"token-name", "", "Looks up the token by name and generates the config using the token (uses default if empty)",
		)

		c.MarkFlagsMutuallyExclusive("token-id", "token-name")
	}

	addConfigFlags(joinTokenMachineConfigCmd)
	addConfigFlags(joinTokenKernelArgsCmd)

	joinTokenRenewCmd.MarkFlagRequired("ttl") //nolint:errcheck
}

func checkTokenWarnings(ctx context.Context, client *client.Client, id, operation string) error {
	joinTokenStatus, err := safe.ReaderGetByID[*siderolink.JoinTokenStatus](ctx, client.Omni().State(), id)
	if err != nil {
		return err
	}

	yellow := color.New(color.FgYellow)

	if joinTokenStatus.TypedSpec().Value.Warnings != nil {
		if _, err = yellow.Fprintf(
			os.Stderr,
			"WARNING: %d of %s won't be able to connect if the token is revoked/deleted\n",
			len(joinTokenStatus.TypedSpec().Value.Warnings),
			pluralize.NewClient().Pluralize("machine", int(joinTokenStatus.TypedSpec().Value.UseCount), true)); err != nil {
			return err
		}

		writer := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)

		if _, err = fmt.Fprintf(writer, "MACHINE\tDETAILS\n"); err != nil {
			return err
		}

		for _, warning := range joinTokenStatus.TypedSpec().Value.Warnings {
			if _, err = fmt.Fprintf(writer, "%s\t%s\n", warning.Machine, warning.Message); err != nil {
				return err
			}
		}

		if err = writer.Flush(); err != nil {
			return err
		}

		var confirmed bool

		confirmed, err = askConfirmation(fmt.Sprintf("Do you still want to %s the token?", operation))
		if err != nil {
			return err
		}

		if !confirmed {
			return errors.New("operation was aborted")
		}
	}

	return nil
}

func askConfirmation(prompt string) (bool, error) {
	if joinTokenDeleteFlags.force || joinTokenRevokeFlags.force {
		return true, nil
	}

	reader := bufio.NewReader(os.Stdin)

	fmt.Printf("%s [y/N]: ", prompt)

	response, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}

	if strings.ToLower(strings.TrimSpace(response)) == "y" {
		return true, nil
	}

	return false, nil
}

func getJoinToken(ctx context.Context, client *client.Client) (string, error) {
	tokenID := joinTokenMachineJoinConfigFlags.tokenID
	if tokenID == "" {
		// fallback to the default join token
		if joinTokenMachineJoinConfigFlags.tokenName == "" {
			defaultJoinToken, err := safe.ReaderGetByID[*siderolink.DefaultJoinToken](ctx, client.Omni().State(), siderolink.DefaultJoinTokenID)
			if err != nil {
				return "", err
			}

			return defaultJoinToken.TypedSpec().Value.TokenId, nil
		}

		// find token by name
		tokens, err := safe.ReaderListAll[*siderolink.JoinToken](ctx, client.Omni().State())
		if err != nil {
			return "", err
		}

		for token := range tokens.All() {
			if token.TypedSpec().Value.Name == joinTokenMachineJoinConfigFlags.tokenName {
				return token.Metadata().ID(), nil
			}

			if joinTokenMachineJoinConfigFlags.tokenName == "initial" && token.TypedSpec().Value.Name == "" {
				return token.Metadata().ID(), nil
			}
		}

		return "", fmt.Errorf("join token with name %q was not found", joinTokenMachineJoinConfigFlags.tokenName)
	}

	_, err := safe.ReaderGetByID[*siderolink.JoinToken](ctx, client.Omni().State(), tokenID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return "", fmt.Errorf("join token with ID %q was not found", tokenID)
		}

		return "", err
	}

	return tokenID, nil
}
