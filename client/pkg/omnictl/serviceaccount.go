// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package omnictl

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"text/tabwriter"
	"time"

	"github.com/siderolabs/go-api-signature/pkg/pgp"
	"github.com/siderolabs/go-api-signature/pkg/serviceaccount"
	"github.com/spf13/cobra"

	pkgaccess "github.com/siderolabs/omni/client/pkg/access"
	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/omnictl/internal/access"
)

var (
	serviceAccountCreateFlags struct {
		role string

		useUserRole bool
		ttl         time.Duration
	}

	serviceAccountRenewFlags struct {
		ttl time.Duration
	}

	// serviceAccountCmd represents the serviceaccount command.
	serviceAccountCmd = &cobra.Command{
		Use:     "serviceaccount",
		Aliases: []string{"sa"},
		Short:   "Manage service accounts",
	}

	serviceAccountCreateCmd = &cobra.Command{
		Use:     "create <name>",
		Aliases: []string{"c"},
		Short:   "Create a service account",
		Args:    cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			name := args[0]

			return access.WithClient(func(ctx context.Context, client *client.Client) error {
				key, err := generateServiceAccountPGPKey(name)
				if err != nil {
					return err
				}

				armoredPublicKey, err := key.ArmorPublic()
				if err != nil {
					return err
				}

				publicKeyID, err := client.Management().CreateServiceAccount(ctx, name, armoredPublicKey, serviceAccountCreateFlags.role, serviceAccountCreateFlags.useUserRole)
				if err != nil {
					return err
				}

				encodedKey, err := serviceaccount.Encode(name, key)
				if err != nil {
					return err
				}

				fmt.Printf("Created service account %q with public key ID %q\n", name, publicKeyID)
				fmt.Printf("\n")
				fmt.Printf("Set the following environment variables to use the service account:\n")
				fmt.Printf("%s=%s\n", access.EndpointEnvVar, client.Endpoint())
				fmt.Printf("%s=%s\n", serviceaccount.OmniServiceAccountKeyEnvVar, encodedKey)
				fmt.Printf("\n")
				fmt.Printf("Note: Store the service account key securely, it will not be displayed again\n")

				return nil
			})
		},
	}

	serviceAccountRenewCmd = &cobra.Command{
		Use:     "renew <name>",
		Aliases: []string{"r"},
		Short:   "Renew a service account by registering a new public key to it",
		Args:    cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			name := args[0]

			return access.WithClient(func(ctx context.Context, client *client.Client) error {
				key, err := generateServiceAccountPGPKey(name)
				if err != nil {
					return err
				}

				armoredPublicKey, err := key.ArmorPublic()
				if err != nil {
					return err
				}

				publicKeyID, err := client.Management().RenewServiceAccount(ctx, name, armoredPublicKey)
				if err != nil {
					return err
				}

				encodedKey, err := serviceaccount.Encode(name, key)
				if err != nil {
					return err
				}

				fmt.Printf("Renewed service account %q by adding a public key with ID %q\n", name, publicKeyID)
				fmt.Printf("\n")
				fmt.Printf("Set the following environment variables to use the service account:\n")
				fmt.Printf("%s=%s\n", access.EndpointEnvVar, client.Endpoint())
				fmt.Printf("%s=%s\n", serviceaccount.OmniServiceAccountKeyEnvVar, encodedKey)
				fmt.Printf("\n")
				fmt.Printf("Note: Store the service account key securely, it will not be displayed again\n")

				return nil
			})
		},
	}

	serviceAccountListCmd = &cobra.Command{
		Use:     "list",
		Aliases: []string{"l"},
		Short:   "List service accounts",
		Args:    cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			return access.WithClient(func(ctx context.Context, client *client.Client) error {
				serviceAccounts, err := client.Management().ListServiceAccounts(ctx)
				if err != nil {
					return err
				}

				writer := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)

				fmt.Fprintf(writer, "NAME\tROLE\tPUBLIC KEY ID\tEXPIRATION\n") //nolint:errcheck

				for _, sa := range serviceAccounts {
					for i, publicKey := range sa.PgpPublicKeys {
						if i == 0 {
							fmt.Fprintf(writer, "%s\t%s\t%s\t%s\n", sa.Name, sa.GetRole(), publicKey.Id, publicKey.Expiration.AsTime().String()) //nolint:errcheck
						} else {
							fmt.Fprintf(writer, "\t\t%s\t%s\n", publicKey.Id, publicKey.Expiration.AsTime().String()) //nolint:errcheck
						}
					}
				}

				return writer.Flush()
			})
		},
	}

	serviceAccountDestroyCmd = &cobra.Command{
		Use:     "destroy <name>",
		Aliases: []string{"d"},
		Short:   "Destroy a service account",
		Args:    cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			name := args[0]

			return access.WithClient(func(ctx context.Context, client *client.Client) error {
				err := client.Management().DestroyServiceAccount(ctx, name)
				if err != nil {
					return fmt.Errorf("failed to destroy service account: %w", err)
				}

				fmt.Printf("destroyed service account: %s\n", name)

				return nil
			})
		},
	}
)

func generateServiceAccountPGPKey(name string) (*pgp.Key, error) {
	comment := fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)
	sa := pkgaccess.ParseServiceAccountFromName(name)
	email := sa.FullID()

	return pgp.GenerateKey(sa.BaseName, comment, email, serviceAccountCreateFlags.ttl)
}

func init() {
	RootCmd.AddCommand(serviceAccountCmd)

	serviceAccountCmd.AddCommand(serviceAccountCreateCmd)
	serviceAccountCmd.AddCommand(serviceAccountListCmd)
	serviceAccountCmd.AddCommand(serviceAccountDestroyCmd)
	serviceAccountCmd.AddCommand(serviceAccountRenewCmd)

	roleFlag := "role"
	useUserRoleFlag := "use-user-role"

	serviceAccountCreateCmd.Flags().DurationVarP(&serviceAccountCreateFlags.ttl, "ttl", "t", 365*24*time.Hour, "TTL for the service account key")
	serviceAccountCreateCmd.Flags().StringVarP(&serviceAccountCreateFlags.role, roleFlag, "r", "", "role of the service account. only used when --"+useUserRoleFlag+"=false")
	serviceAccountCreateCmd.Flags().BoolVarP(&serviceAccountCreateFlags.useUserRole, useUserRoleFlag, "u", true, "use the role of the creating user. if true, --"+roleFlag+" is ignored")

	serviceAccountRenewCmd.Flags().DurationVarP(&serviceAccountRenewFlags.ttl, "ttl", "t", 365*24*time.Hour, "TTL for the service account key")
}
