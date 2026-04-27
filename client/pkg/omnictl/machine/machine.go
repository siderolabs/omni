// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package machine contains machine-level commands for omnictl.
package machine

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/omnictl/internal/access"
)

var shutdownCmd = &cobra.Command{
	Use:   "shutdown machine-id",
	Short: "Shut down a machine",
	Long: `Shut down a machine. For machines managed by an infra provider, this also prevents
the provider from automatically powering the machine back on.`,
	Args: cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		return access.WithClient(func(ctx context.Context, client *client.Client, _ access.ServerInfo) error {
			if err := client.Management().MachinePowerOff(ctx, args[0]); err != nil {
				return fmt.Errorf("failed to shut down machine %q: %w", args[0], err)
			}

			fmt.Fprintf(os.Stderr, "machine %q is shutting down\n", args[0])

			return nil
		})
	},
}

var powerOnCmd = &cobra.Command{
	Use:   "power-on machine-id",
	Short: "Power on a machine",
	Long: `Power on a machine managed by an infra provider. This clears the power off request
and allows the provider to power the machine on.`,
	Args: cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		return access.WithClient(func(ctx context.Context, client *client.Client, _ access.ServerInfo) error {
			if err := client.Management().MachinePowerOn(ctx, args[0]); err != nil {
				return fmt.Errorf("failed to power on machine %q: %w", args[0], err)
			}

			fmt.Fprintf(os.Stderr, "machine %q power on requested\n", args[0])

			return nil
		})
	},
}

// RootCmd returns the root machine command.
func RootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:     "machine",
		Short:   "Machine related commands.",
		Long:    `Commands to manage machines.`,
		Example: "",
	}

	rootCmd.AddCommand(shutdownCmd)
	rootCmd.AddCommand(powerOnCmd)

	return rootCmd
}
