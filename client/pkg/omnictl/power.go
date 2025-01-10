// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package omnictl

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	managementpb "github.com/siderolabs/omni/client/api/omni/management"
	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/omnictl/internal/access"
)

var (
	powerCmd = &cobra.Command{
		Use:   "power",
		Short: "Manage power state of infrastructure machines",
	}

	powerRebootCmd = &cobra.Command{
		Use:     "reboot <machine-id>",
		Aliases: []string{"cycle"},
		Short:   "Reboot a machine",
		Args:    cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			machineID := args[0]

			return access.WithClient(func(ctx context.Context, client *client.Client) error {
				resp, err := client.Management().RebootMachine(ctx, &managementpb.RebootMachineRequest{
					MachineId: machineID,
				})
				if err != nil {
					return err
				}

				fmt.Printf("Rebooted machine: %s\n", machineID)
				fmt.Printf("Reboot ID: %s\n", resp.RebootId)
				fmt.Printf("Reboot time: %s\n", resp.LastRebootTimestamp.AsTime().String())

				return nil
			})
		},
	}
)

func init() {
	RootCmd.AddCommand(powerCmd)

	powerCmd.AddCommand(powerRebootCmd)
}
