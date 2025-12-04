// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package machine

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/siderolink/pkg/wireguard"
	"github.com/spf13/cobra"

	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/client/pkg/omnictl/internal/access"
)

const (
	tunnelModeDisabled = "disabled"
	tunnelModeEnabled  = "enabled"
	tunnelModeAuto     = "auto"
)

// getCmd represents the get logs command.
var configureCmd = &cobra.Command{
	Use:     "configure",
	Aliases: []string{"c"},
	Short:   "Configure machine",
	Long:    `Configure machine by id`,
}

var delayNotice = fmt.Sprintf("It will take around %s for the machine to reconnect back in the new mode.", wireguard.PeerDownInterval)

var configureTunnel = &cobra.Command{
	Use:     "grpc-tunnel-mode",
	Aliases: []string{"t"},
	Short:   "Configure machine",
	RunE: func(cmd *cobra.Command, args []string) error {
		return access.WithClient(func(ctx context.Context, client *client.Client) error {
			mode := args[0]

			_, err := safe.ReaderGetByID[*omni.Machine](ctx, client.Omni().State(), machineCmdFlags.machine)
			if err != nil {
				if state.IsNotFoundError(err) {
					return fmt.Errorf("machine with id %s doesn't exist", machineCmdFlags.machine)
				}

				return err
			}

			config := siderolink.NewGRPCTunnelConfig(machineCmdFlags.machine)

			switch mode {
			case tunnelModeDisabled:
				return safe.StateModify(ctx, client.Omni().State(), config, func(res *siderolink.GRPCTunnelConfig) error {
					if !res.TypedSpec().Value.Enabled {
						return nil
					}

					res.TypedSpec().Value.Enabled = false

					fmt.Printf("disabled gRPC tunnel mode for machine %s\n%s", machineCmdFlags.machine, delayNotice)

					return nil
				})
			case tunnelModeEnabled:
				return safe.StateModify(ctx, client.Omni().State(), config, func(res *siderolink.GRPCTunnelConfig) error {
					if res.TypedSpec().Value.Enabled {
						return nil
					}

					res.TypedSpec().Value.Enabled = true

					fmt.Printf("enabled gRPC tunnel mode for machine %s\n%s", machineCmdFlags.machine, delayNotice)

					return nil
				})

			case tunnelModeAuto:
				err = client.Omni().State().TeardownAndDestroy(ctx, config.Metadata())
				if err != nil && !state.IsNotFoundError(err) {
					return err
				}

				fmt.Printf("deleted grpc tunnel config for the machine %s\n%s", machineCmdFlags.machine, delayNotice)
			default:
				return fmt.Errorf("unknown mode %s, should be one of %q", mode, []string{tunnelModeDisabled, tunnelModeEnabled, tunnelModeAuto})
			}

			fmt.Printf("WARNING: the changes won't be applied until the machine is restarted")

			return nil
		})
	},
	Args: cobra.ExactArgs(1),
}

func init() {
	configureCmd.AddCommand(configureTunnel)

	machineCmd.AddCommand(configureCmd)
}
