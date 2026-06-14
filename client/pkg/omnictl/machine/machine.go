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

	"github.com/siderolabs/omni/client/api/omni/management"
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

var installCmdFlags struct {
	version string
	disk    string
}

var installCmd = &cobra.Command{
	Use:   "install machine-id",
	Short: "Install Talos to disk on a machine running in maintenance mode",
	Long: `Install Talos to disk on a machine that is running in maintenance mode.

This requires the machine to be running Talos 1.13 or newer. The target disk must be specified
with --disk (e.g. /dev/sda); use 'omnictl get machinestatus <machine-id> -o yaml' to inspect the
available block devices. When --version is omitted, the version Talos is currently running in
memory on the machine is installed. Installer progress is streamed to the terminal; on success the
machine is rebooted so it boots from disk into the freshly installed Talos.`,
	Args: cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		if installCmdFlags.disk == "" {
			return fmt.Errorf("--disk is required")
		}

		return access.WithClient(func(ctx context.Context, client *client.Client, _ access.ServerInfo) error {
			lifecycle := client.Management().MaintenanceLifecycle(
				ctx,
				args[0],
				management.MaintenanceLifecycleRequest_OPERATION_INSTALL,
				installCmdFlags.version,
				installCmdFlags.disk,
			)

			for resp, err := range lifecycle {
				if err != nil {
					return fmt.Errorf("failed to install Talos on machine %q: %w", args[0], err)
				}

				if msg := resp.GetMessage(); msg != "" {
					fmt.Fprintln(os.Stderr, msg)
				}
			}

			fmt.Fprintf(os.Stderr, "machine %q: Talos installed to %s\n", args[0], installCmdFlags.disk)

			return nil
		})
	},
}

var upgradeCmdFlags struct {
	version string
}

var upgradeCmd = &cobra.Command{
	Use:   "upgrade machine-id",
	Short: "Upgrade the on-disk Talos on a machine running in maintenance mode",
	Long: `Upgrade the on-disk Talos on a machine that is running in maintenance mode and already has Talos installed.

This requires the machine to be running Talos 1.13 or newer. --version is required and specifies the Talos version to write to disk. 
Installer progress is streamed to the terminal; on success the machine is rebooted so it boots from disk into the upgraded Talos.`,
	Args: cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		if upgradeCmdFlags.version == "" {
			return fmt.Errorf("--version is required")
		}

		return access.WithClient(func(ctx context.Context, client *client.Client, _ access.ServerInfo) error {
			lifecycle := client.Management().MaintenanceLifecycle(
				ctx,
				args[0],
				management.MaintenanceLifecycleRequest_OPERATION_UPGRADE,
				upgradeCmdFlags.version,
				"",
			)

			for resp, err := range lifecycle {
				if err != nil {
					return fmt.Errorf("failed to upgrade Talos on machine %q: %w", args[0], err)
				}

				if msg := resp.GetMessage(); msg != "" {
					fmt.Fprintln(os.Stderr, msg)
				}
			}

			fmt.Fprintf(os.Stderr, "machine %q: Talos upgraded\n", args[0])

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
	rootCmd.AddCommand(installCmd)
	rootCmd.AddCommand(upgradeCmd)

	return rootCmd
}

func init() {
	installCmd.Flags().StringVar(&installCmdFlags.version, "version", "", "Talos version to install; defaults to the version running in memory on the machine")
	installCmd.Flags().StringVar(&installCmdFlags.disk, "disk", "", "target install disk (e.g. /dev/sda)")

	upgradeCmd.Flags().StringVar(&upgradeCmdFlags.version, "version", "", "Talos version to upgrade to")
}
