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
	"github.com/siderolabs/go-api-signature/pkg/serviceaccount"
	"github.com/spf13/cobra"

	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omnictl/internal/access"
)

var (
	infraProviderCreateFlags struct {
		ttl time.Duration
	}

	infraProviderRenewFlags struct {
		ttl time.Duration
	}

	// infraProviderCmd represents the infraprovider command.
	infraProviderCmd = &cobra.Command{
		Use:     "infraprovider",
		Aliases: []string{"ip"},
		Short:   "Manage infra providers",
	}

	infraProviderCreateCmd = &cobra.Command{
		Use:     "create <name>",
		Aliases: []string{"c"},
		Short:   "Create an infra provider",
		Args:    cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			name := args[0]

			return access.WithClient(func(ctx context.Context, client *client.Client) error {
				infraProvider := infra.NewProvider(name)

				err := client.Omni().State().Create(ctx, infraProvider)
				if err != nil {
					return err
				}

				serviceAccountName := fmt.Sprintf("infra-provider:%s", name)

				key, err := generateServiceAccountPGPKey(serviceAccountName)
				if err != nil {
					return err
				}

				armoredPublicKey, err := key.ArmorPublic()
				if err != nil {
					return err
				}

				publicKeyID, err := client.Management().CreateServiceAccount(ctx, serviceAccountName, armoredPublicKey, "InfraProvider", false)
				if err != nil {
					return err
				}

				encodedKey, err := serviceaccount.Encode(serviceAccountName, key)
				if err != nil {
					return err
				}

				fmt.Printf("Your infra provider %q is ready to use\n", name)
				fmt.Printf("Created infra provider service account %q with public key ID %q\n", serviceAccountName, publicKeyID)
				fmt.Printf("\n")
				fmt.Printf("Set the following environment variables to use the service account:\n")
				fmt.Printf("%s=%s\n", access.EndpointEnvVar, client.Endpoint())
				fmt.Printf("%s=%s\n", serviceaccount.OmniServiceAccountKeyEnvVar, encodedKey)
				fmt.Println("")
				fmt.Println("Note: Store the service account key securely, it will not be displayed again")
				fmt.Println("Please use the endpoint and the service account key to set up the infra provider")

				return nil
			})
		},
	}

	infraProviderRenewKeyCmd = &cobra.Command{
		Use:     "renewkey <name>",
		Aliases: []string{"r"},
		Short:   "Renew an infra provider service account by registering a new public key to it",
		Args:    cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			name := args[0]

			return access.WithClient(func(ctx context.Context, client *client.Client) error {
				serviceAccountName := fmt.Sprintf("infra-provider:%s", name)

				key, err := generateServiceAccountPGPKey(serviceAccountName)
				if err != nil {
					return err
				}

				armoredPublicKey, err := key.ArmorPublic()
				if err != nil {
					return err
				}

				publicKeyID, err := client.Management().RenewServiceAccount(ctx, serviceAccountName, armoredPublicKey)
				if err != nil {
					return err
				}

				encodedKey, err := serviceaccount.Encode(serviceAccountName, key)
				if err != nil {
					return err
				}

				fmt.Printf("Renewed service account %q by adding a public key with ID %q\n", serviceAccountName, publicKeyID)
				fmt.Printf("\n")
				fmt.Printf("Set the following environment variables to use the service account:\n")
				fmt.Printf("%s=%s\n", access.EndpointEnvVar, client.Endpoint())
				fmt.Printf("%s=%s\n", serviceaccount.OmniServiceAccountKeyEnvVar, encodedKey)
				fmt.Println("")
				fmt.Println("Note: Store the service account key securely, it will not be displayed again")

				return nil
			})
		},
	}

	infraProviderListCmd = &cobra.Command{
		Use:     "list",
		Aliases: []string{"l"},
		Short:   "List infra providers",
		Args:    cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			return access.WithClient(func(ctx context.Context, client *client.Client) error {
				infraProviders, err := safe.StateListAll[*omni.InfraProviderCombinedStatus](ctx, client.Omni().State())
				if err != nil {
					return err
				}

				writer := tabwriter.NewWriter(os.Stdout, 0, 0, 4, ' ', 0)

				fmt.Fprintf(writer, "ID\tNAME\tDESCRIPTION\tCONNECTED\tERROR\n") //nolint:errcheck

				for ps := range infraProviders.All() {
					fmt.Fprintf(writer, //nolint:errcheck
						"%s\t%s\t%s\t%t\t%s\n",
						ps.Metadata().ID(),
						ps.TypedSpec().Value.Name,
						ps.TypedSpec().Value.Description,
						ps.TypedSpec().Value.Health.Connected,
						ps.TypedSpec().Value.Health.Error,
					)
				}

				return writer.Flush()
			})
		},
	}

	infraProviderDeleteCmd = &cobra.Command{
		Use:     "delete <name>",
		Aliases: []string{"d"},
		Short:   "Delete an infra provider",
		Args:    cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			name := args[0]

			return access.WithClient(func(ctx context.Context, client *client.Client) error {
				err := client.Omni().State().TeardownAndDestroy(ctx, infra.NewProvider(name).Metadata())
				if err != nil {
					return fmt.Errorf("failed to delete infra provider: %w", err)
				}

				fmt.Printf("deleted infra provider: %s\n", name)

				return nil
			})
		},
	}
)

func init() {
	RootCmd.AddCommand(infraProviderCmd)

	infraProviderCmd.AddCommand(infraProviderCreateCmd)
	infraProviderCmd.AddCommand(infraProviderListCmd)
	infraProviderCmd.AddCommand(infraProviderDeleteCmd)
	infraProviderCmd.AddCommand(infraProviderRenewKeyCmd)

	infraProviderCreateCmd.Flags().DurationVarP(&infraProviderCreateFlags.ttl, "ttl", "t", 365*24*time.Hour, "TTL for the infra provider service account key")

	infraProviderRenewKeyCmd.Flags().DurationVarP(&infraProviderRenewFlags.ttl, "ttl", "t", 365*24*time.Hour, "TTL for the infra provider service account key")
}
