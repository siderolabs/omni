// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package tests

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/go-retry/retry"
	talosclient "github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// HelloWorldServiceExtensionName is the name of the sample hello world extension used for testing.
const HelloWorldServiceExtensionName = "hello-world-service"

// QemuGuestAgentExtensionName is the name of the qemu guest agent extension used for testing.
const QemuGuestAgentExtensionName = "qemu-guest-agent"

// AssertExtensionIsPresent asserts that the extension "hello-world-service" is present on all machines in the cluster.
func AssertExtensionIsPresent(ctx context.Context, cli *client.Client, cluster, extension string) TestFunc {
	return func(t *testing.T) {
		clusterMachineList, err := safe.StateListAll[*omni.ClusterMachine](ctx, cli.Omni().State(), state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, cluster)))
		require.NoError(t, err)

		machineIDs := make([]resource.ID, 0, clusterMachineList.Len())

		clusterMachineList.ForEach(func(clusterMachine *omni.ClusterMachine) {
			machineIDs = append(machineIDs, clusterMachine.Metadata().ID())
		})

		checkExtensionWithRetries(ctx, t, cli, extension, machineIDs...)
	}
}

func checkExtensionWithRetries(ctx context.Context, t *testing.T, cli *client.Client, extension string, machineIDs ...resource.ID) {
	for _, machineID := range machineIDs {
		numErrs := 0

		err := retry.Constant(3*time.Minute, retry.WithUnits(time.Second), retry.WithAttemptTimeout(3*time.Second)).RetryWithContext(ctx, func(ctx context.Context) error {
			if err := checkExtension(ctx, cli, machineID, extension); err != nil {
				numErrs++

				if numErrs%10 == 0 {
					t.Logf("failed to check extension %q on machine %q: %v", extension, machineID, err)
				}

				return retry.ExpectedError(err)
			}

			t.Logf("found extension %q on machine %q", extension, machineID)

			return nil
		})
		require.NoError(t, err)
	}
}

func checkExtension(ctx context.Context, cli *client.Client, machineID resource.ID, extension string) error {
	machineStatus, err := safe.StateGet[*omni.MachineStatus](ctx, cli.Omni().State(), omni.NewMachineStatus(resources.DefaultNamespace, machineID).Metadata())
	if err != nil {
		return err
	}

	var talosCli *talosclient.Client

	if machineStatus.TypedSpec().Value.GetMaintenance() {
		if talosCli, err = talosClientMaintenance(ctx, machineStatus.TypedSpec().Value.GetManagementAddress()); err != nil {
			return err
		}
	} else {
		cluster, ok := machineStatus.Metadata().Labels().Get(omni.LabelCluster)
		if !ok {
			return fmt.Errorf("machine %q is not in maintenance mode but does not have a cluster label", machineStatus.Metadata().ID())
		}

		if talosCli, err = talosClient(ctx, cli, cluster); err != nil {
			return err
		}
	}

	extensionStatusList, err := safe.StateListAll[*runtime.ExtensionStatus](ctx, talosCli.COSI)
	if err != nil {
		return err
	}

	for extensionStatus := range extensionStatusList.All() {
		if extensionStatus.TypedSpec().Metadata.Name == extension {
			return nil
		}
	}

	return fmt.Errorf("extension %q is not found on machine %q", extension, machineStatus.Metadata().ID())
}
