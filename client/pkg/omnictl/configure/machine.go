// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package configure

import (
	"context"
	"fmt"
	"slices"

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
	siderolinkConnectionModeUDP    = "udp"
	siderolinkConnectionModeTunnel = "http-tunnel"
	siderolinkConnectionModeAuto   = "auto"
)

var machineCmdFlags struct {
	siderolinkConnection siderolinkConnection
	resetNodeUniqueToken bool
}

var delayNotice = fmt.Sprintf("It will take around %s for the machine to reconnect back in the new mode.\n", wireguard.PeerDownInterval)

// machineCmd represents the machine configure command.
var machineCmd = &cobra.Command{
	Use:     "machine <uuid1> <uuid2>",
	Aliases: []string{"m"},
	Short:   "Machine configuration command.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return access.WithClient(func(ctx context.Context, client *client.Client) error {
			for _, id := range args {
				if machineCmdFlags.siderolinkConnection != "" {
					if err := updateSideroLinkConnectionMode(ctx, client, id); err != nil {
						return fmt.Errorf("failed to update SideroLink connection mode for machine %s: %w", id, err)
					}
				}

				if machineCmdFlags.resetNodeUniqueToken {
					err := client.Management().ResetNodeUniqueToken(ctx, id)
					if err != nil {
						return fmt.Errorf("failed to reset node unique token for machine %s: %w", id, err)
					}

					fmt.Printf("reset node unique token for machine %s\n", id)
				}
			}

			return nil
		})
	},
	Args: cobra.MinimumNArgs(1),
}

func updateSideroLinkConnectionMode(ctx context.Context, client *client.Client, id string) error {
	_, err := safe.ReaderGetByID[*omni.Machine](ctx, client.Omni().State(), id)
	if err != nil {
		if state.IsNotFoundError(err) {
			return fmt.Errorf("machine with id %s doesn't exist", id)
		}

		return err
	}

	config := siderolink.NewGRPCTunnelConfig(id)

	switch string(machineCmdFlags.siderolinkConnection) {
	case siderolinkConnectionModeUDP:
		return safe.StateModify(ctx, client.Omni().State(), config, func(res *siderolink.GRPCTunnelConfig) error {
			if !res.TypedSpec().Value.Enabled {
				return nil
			}

			res.TypedSpec().Value.Enabled = false

			fmt.Printf("disabled gRPC tunnel mode for machine %s\n%s", id, delayNotice)

			return nil
		})
	case siderolinkConnectionModeTunnel:
		return safe.StateModify(ctx, client.Omni().State(), config, func(res *siderolink.GRPCTunnelConfig) error {
			if res.TypedSpec().Value.Enabled {
				return nil
			}

			res.TypedSpec().Value.Enabled = true

			fmt.Printf("enabled gRPC tunnel mode for machine %s\n%s", id, delayNotice)

			return nil
		})

	case siderolinkConnectionModeAuto:
		err = client.Omni().State().TeardownAndDestroy(ctx, config.Metadata())
		if err != nil && !state.IsNotFoundError(err) {
			return err
		}

		fmt.Printf("deleted grpc tunnel config for the machine %s\n%s", id, delayNotice)
	}

	fmt.Println("WARNING: the changes won't be applied until the machine is restarted")

	return nil
}

type siderolinkConnection string

// Set implements pflag.Value.
func (s *siderolinkConnection) Set(value string) error {
	if !slices.Contains(
		[]string{siderolinkConnectionModeAuto, siderolinkConnectionModeTunnel, siderolinkConnectionModeUDP}, value,
	) {
		return fmt.Errorf("unknown mode %q, should be %s", value, s.Type())
	}

	*s = siderolinkConnection(value)

	return nil
}

// Type implements pflag.Value.
func (*siderolinkConnection) Type() string {
	return "one of [udp,http-tunnel,auto]"
}

// String implements pflag.Value.
func (s *siderolinkConnection) String() string {
	return string(*s)
}

func init() {
	machineCmd.Flags().Var(&machineCmdFlags.siderolinkConnection, "siderolink-connection", "configure SideroLink connection mode")
	machineCmd.Flags().BoolVar(&machineCmdFlags.resetNodeUniqueToken, "reset-node-unique-token", false, "reset the node unique token for the machine")

	configureCmd.AddCommand(machineCmd)
}
