// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/siderolabs/omni/client/pkg/client"
	omniresources "github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/client/pkg/omnictl/internal/access"
	"github.com/siderolabs/omni/client/pkg/omnictl/resources"
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

var machineDeleteCmdFlags struct {
	timeout        time.Duration
	forceEtcdLeave bool
	force          bool
	noAsk          bool
}

var machineDeleteCmd = &cobra.Command{
	Use:     "delete machine-id",
	Short:   "Delete the machine from the cluster",
	Long:    `Delete the machine from the cluster. The command waits for the machine to be fully deleted.`,
	Aliases: []string{"rm", "destroy"},
	Args:    cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		return access.WithClient(func(ctx context.Context, client *client.Client, _ access.ServerInfo) error {
			return deleteMachine(ctx, client.Omni().State(), args[0])
		})
	},
}

func setLocked(machineID resource.ID, lock bool) func(context.Context, *client.Client, access.ServerInfo) error {
	return func(ctx context.Context, client *client.Client, _ access.ServerInfo) error {
		st := client.Omni().State()

		machineSetNode, err := safe.StateGet[*omni.MachineSetNode](ctx, st, resource.NewMetadata(omniresources.DefaultNamespace, omni.MachineSetNodeType, machineID, resource.VersionUndefined))
		if err != nil {
			return err
		}

		_, err = safe.StateUpdateWithConflicts(ctx, st, machineSetNode.Metadata(), func(res *omni.MachineSetNode) error {
			if lock {
				res.Metadata().Annotations().Set(omni.MachineLocked, "")
			} else {
				res.Metadata().Annotations().Delete(omni.MachineLocked)
			}

			return nil
		})
		if err != nil {
			return err
		}

		fmt.Fprintf(os.Stderr, "machine %q lock status: %t\n", machineID, lock)

		return nil
	}
}

//nolint:gocognit,gocyclo,cyclop
func deleteMachine(ctx context.Context, st state.State, id resource.ID) error {
	ctx, cancel := context.WithTimeout(ctx, machineDeleteCmdFlags.timeout)
	defer cancel()

	clusterMachineMD := omni.NewClusterMachine(id).Metadata()

	clusterMachine, err := st.Get(ctx, clusterMachineMD)
	if err != nil {
		return err
	}

	yellow := color.New(color.FgYellow)

	if machineDeleteCmdFlags.forceEtcdLeave {
		if !machineDeleteCmdFlags.noAsk {
			var response string

			//nolint:errcheck
			yellow.Fprintf(os.Stderr, `WARNING: force leaving etcd member on machine %s may break the etcd quorum.
Do not use this option unless you are sure that the machine is not an etcd member or the machine is already down and will not come back up.
`, id)
			fmt.Fprint(os.Stderr, `Do you want to continue? (y/N): `)

			_, err = fmt.Scanln(&response)
			if err != nil {
				return fmt.Errorf("failed to read user input: %w", err)
			}

			if !strings.EqualFold(response, "y") {
				fmt.Fprintln(os.Stderr, "aborting machine deletion")

				return nil
			}
		}

		fmt.Fprintf(os.Stderr, "create %s for machine %s\n\n", omni.NodeForceDestroyRequestType, id)

		forceDestroyRequest := omni.NewNodeForceDestroyRequest(id)

		cluster, ok := clusterMachine.Metadata().Labels().Get(omni.LabelCluster)
		if !ok {
			return fmt.Errorf("failed to get cluster label from %s %s", omni.ClusterMachineType, id)
		}

		forceDestroyRequest.Metadata().Labels().Set(omni.LabelCluster, cluster)

		if err = st.Create(ctx, forceDestroyRequest); err != nil && !state.IsConflictError(err) {
			return fmt.Errorf("failed to create %s for machine %s: %w", omni.NodeForceDestroyRequestType, id, err)
		}
	}

	if machineDeleteCmdFlags.force {
		if !machineDeleteCmdFlags.noAsk {
			var response string

			//nolint:errcheck
			yellow.Fprintf(os.Stderr, `WARNING: force deleting machine %s will skip wiping the machine and will remove it from the Omni account completely.
It will be necessary to manually wipe the machine to add it to the Omni account again.
Use this option only in case if the machine is already down and will not come back up.
`, id)

			fmt.Fprint(os.Stderr, `Do you want to continue? (y/N): `)

			_, err = fmt.Scanln(&response)
			if err != nil {
				return fmt.Errorf("failed to read user input: %w", err)
			}

			if !strings.EqualFold(response, "y") {
				fmt.Fprintln(os.Stderr, "aborting machine deletion")

				return nil
			}
		}

		if _, err = st.Teardown(ctx, siderolink.NewLink(clusterMachine.Metadata().ID(), nil).Metadata()); err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("failed to teardown %s for machine %s: %w", siderolink.LinkType, id, err)
		}

		fmt.Fprintf(os.Stderr, "teardown %s %s\n\n", siderolink.LinkType, id)
	}

	machineSetNode, err := safe.StateGetByID[*omni.MachineSetNode](ctx, st, id)
	if err != nil && !state.IsNotFoundError(err) {
		return fmt.Errorf("failed to get %s %s: %w", omni.MachineSetNodeType, id, err)
	}

	if machineSetNode != nil {
		fmt.Fprintf(os.Stderr, "destroy %s %s\n", omni.MachineSetNodeType, id)

		if err = resources.Destroy(ctx, st, "", omni.MachineSetNodeType, "", false, []resource.ID{id}); err != nil && !state.IsNotFoundError(err) {
			return err
		}
	}

	fmt.Fprintf(os.Stderr, "wait until %s %s is destroyed\n", omni.ClusterMachineType, id)

	watchCh := make(chan safe.WrappedStateEvent[*omni.ClusterMachine])

	if err = safe.StateWatch(ctx, st, clusterMachineMD, watchCh); err != nil {
		return fmt.Errorf("failed to establish a watch on %s %s: %w", omni.ClusterMachineType, id, err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event := <-watchCh:
			if err = event.Error(); err != nil {
				return fmt.Errorf("error watching for %s deletion: %w", omni.ClusterMachineType, err)
			}

			if event.Type() == state.Destroyed {
				fmt.Fprintf(os.Stderr, "%s %s is destroyed\n", omni.ClusterMachineType, id)

				return nil
			}
		}
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
	machineDeleteCmd.PersistentFlags().BoolVar(&machineDeleteCmdFlags.forceEtcdLeave, "force-etcd-leave", false, "force leave etcd")
	machineDeleteCmd.PersistentFlags().BoolVarP(&machineDeleteCmdFlags.force, "force", "f", false, "force delete the machine, skipping wipe")
	machineDeleteCmd.PersistentFlags().BoolVarP(&machineDeleteCmdFlags.noAsk, "yes-i-am-really-sure", "y", false, "do not ask for confirmation when using --force-etcd-leave")
	machineDeleteCmd.PersistentFlags().DurationVarP(&machineDeleteCmdFlags.timeout, "timeout", "t", 5*time.Minute, "timeout for the machine deletion")

	machineCmd.AddCommand(lockCmd)
	machineCmd.AddCommand(unlockCmd)
	machineCmd.AddCommand(machineDeleteCmd)

	clusterCmd.AddCommand(machineCmd)
}
