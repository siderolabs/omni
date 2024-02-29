// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/spf13/cobra"

	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omnictl/internal/access"
)

var lockCmd = &cobra.Command{
	Use:   "lock machine-id",
	Short: "Lock the machine",
	Long:  `When locked, no config updates, upgrades and downgrades will be performed on the machine.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		return access.WithClient(setLocked(args[0], true))
	},
}

var unlockCmd = &cobra.Command{
	Use:   "unlock machine-id",
	Short: "Unlock the machine",
	Long:  `Removes locked annotation from the machine.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		return access.WithClient(setLocked(args[0], false))
	},
}

func setLocked(machineID resource.ID, lock bool) func(context.Context, *client.Client) error {
	return func(ctx context.Context, client *client.Client) error {
		st := client.Omni().State()

		machineSetNode, err := safe.StateGet[*omni.MachineSetNode](ctx, st, resource.NewMetadata(resources.DefaultNamespace, omni.MachineSetNodeType, machineID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				fmt.Printf("no machine set nodes with id %q found", machineID)
			}
		}

		_, err = safe.StateUpdateWithConflicts(ctx, st, machineSetNode.Metadata(), func(res *omni.MachineSetNode) error {
			if lock {
				res.Metadata().Annotations().Set(omni.MachineLocked, "")
			} else {
				res.Metadata().Annotations().Delete(omni.MachineLocked)
			}

			return nil
		})

		return err
	}
}

// machineCmd represents the cluster machine commands.
var machineCmd = &cobra.Command{
	Use:     "machine",
	Short:   "Machine related commands.",
	Long:    `Commands to manage cluster machines.`,
	Example: "",
}

func init() {
	machineCmd.AddCommand(lockCmd)
	machineCmd.AddCommand(unlockCmd)
	clusterCmd.AddCommand(machineCmd)
}
