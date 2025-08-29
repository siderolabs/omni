// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build integration

package integration_test

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/go-retry/retry"
	"github.com/siderolabs/image-factory/pkg/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/extensions"
)

// HelloWorldServiceExtensionName is the name of the sample hello world extension used for testing.
const HelloWorldServiceExtensionName = extensions.OfficialPrefix + "hello-world-service"

// AssertExtensionsAreEqual asserts that the given extensions are all present on all machines of the given cluster.
func AssertExtensionsArePresent(ctx context.Context, cli *client.Client, cluster string, extensions []string) TestFunc {
	return func(t *testing.T) {
		clusterMachineList, err := safe.StateListAll[*omni.ClusterMachine](ctx, cli.Omni().State(), state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, cluster)))
		require.NoError(t, err)

		machineIDs := make([]resource.ID, 0, clusterMachineList.Len())

		clusterMachineList.ForEach(func(clusterMachine *omni.ClusterMachine) {
			machineIDs = append(machineIDs, clusterMachine.Metadata().ID())
		})

		checkExtensionsWithRetries(ctx, t, cli, extensions, machineIDs)
	}
}

func checkExtensionsWithRetries(ctx context.Context, t *testing.T, cli *client.Client, extensions []string, machineIDs []resource.ID) {
	for _, machineID := range machineIDs {
		numErrs := 0

		err := retry.Constant(3*time.Minute, retry.WithUnits(time.Second), retry.WithAttemptTimeout(3*time.Second)).RetryWithContext(ctx, func(ctx context.Context) error {
			if err := checkExtensions(ctx, cli, machineID, extensions); err != nil {
				numErrs++

				if numErrs%10 == 0 {
					t.Logf("failed to check extensions on machine %q: %v", machineID, err)
				}

				return retry.ExpectedError(err)
			}

			t.Logf("found extensions %q on machine %q", extensions, machineID)

			return nil
		})
		require.NoError(t, err)
	}
}

// checkExtensions checks that the given extensions are all present on the machine with the given ID.
//
// The order of the extensions is also checked.
//
// It is assumed that neither of the input slices will contain duplicates.
func checkExtensions(ctx context.Context, cli *client.Client, machineID string, extensions []string) error {
	collectedExtensions, err := fetchExtensions(ctx, cli, machineID)
	if err != nil {
		return err
	}

	pos := 0
	for _, ext := range extensions {
		i := slices.Index(collectedExtensions[pos:], ext)
		if i < 0 {
			return fmt.Errorf("extensions/order mismatch on %q: expected %q to be a subsequence of %q", machineID, extensions, collectedExtensions)
		}
		pos += i + 1
	}

	return nil
}

func fetchExtensions(ctx context.Context, cli *client.Client, machineID resource.ID) ([]string, error) {
	talosCli, err := talosClientForMachine(ctx, cli, machineID)
	if err != nil {
		return nil, err
	}

	list, err := safe.StateListAll[*runtime.ExtensionStatus](ctx, talosCli.COSI)
	if err != nil {
		return nil, err
	}

	exts := make([]string, 0, list.Len())

	for extensionStatus := range list.All() {
		name := extensionStatus.TypedSpec().Metadata.Name
		if name == constants.SchematicIDExtensionName {
			continue
		}

		exts = append(exts, extensions.OfficialPrefix+name)
	}

	return exts, nil
}

func AssertExtraKernelArgsArePresent(ctx context.Context, cli *client.Client, cluster string, extraKernelArgs []string) TestFunc {
	return func(t *testing.T) {
		clusterMachineList, err := safe.StateListAll[*omni.ClusterMachine](ctx, cli.Omni().State(), state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, cluster)))
		require.NoError(t, err)

		machineIDs := make([]resource.ID, 0, clusterMachineList.Len())

		clusterMachineList.ForEach(func(clusterMachine *omni.ClusterMachine) {
			machineIDs = append(machineIDs, clusterMachine.Metadata().ID())
		})

		checkExtraKernelArgsWithRetries(ctx, t, cli, extraKernelArgs, machineIDs)
	}
}

func checkExtraKernelArgsWithRetries(ctx context.Context, t *testing.T, cli *client.Client, extraKernelArgs []string, machineIDs []resource.ID) {
	for _, machineID := range machineIDs {
		numErrs := 0

		err := retry.Constant(3*time.Minute, retry.WithUnits(time.Second), retry.WithAttemptTimeout(3*time.Second)).RetryWithContext(ctx, func(ctx context.Context) error {
			if err := checkExtraKernelArgs(ctx, cli, machineID, extraKernelArgs); err != nil {
				numErrs++

				if numErrs%10 == 0 {
					t.Logf("failed to check extra kernel args on machine %q: %v", machineID, err)
				}
				return retry.ExpectedError(err)
			}

			t.Logf("found extra kernel args %q on machine %q", extraKernelArgs, machineID)

			return nil
		})
		require.NoError(t, err)
	}
}

func checkExtraKernelArgs(ctx context.Context, cli *client.Client, machineID resource.ID, extraKernelArgs []string) error {
	kernelCmdline, err := fetchKernelCmdline(ctx, cli, machineID)
	if err != nil {
		return err
	}

	extraKernelArgsStr := strings.Join(extraKernelArgs, " ")
	if !strings.Contains(kernelCmdline, extraKernelArgsStr) {
		return fmt.Errorf("extra kernel args are not present in machine %q: expected to find %q in %q", machineID, extraKernelArgsStr, kernelCmdline)
	}

	return nil
}

func fetchKernelCmdline(ctx context.Context, cli *client.Client, machineID resource.ID) (string, error) {
	talosCli, err := talosClientForMachine(ctx, cli, machineID)
	if err != nil {
		return "", err
	}

	cmdline, err := safe.StateGetByID[*runtime.KernelCmdline](ctx, talosCli.COSI, runtime.KernelCmdlineID)
	if err != nil {
		return "", err
	}

	return cmdline.TypedSpec().Cmdline, nil
}

// UpdateExtensions updates the extensions on all the machines of the given cluster.
func UpdateExtensions(ctx context.Context, cli *client.Client, cluster string, extensions []string) TestFunc {
	return func(t *testing.T) {
		clusterMachineList, err := safe.StateListAll[*omni.ClusterMachine](ctx, cli.Omni().State(), state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, cluster)))
		require.NoError(t, err)

		require.Greater(t, clusterMachineList.Len(), 0)

		for clusterMachine := range clusterMachineList.All() {
			var extensionsConfig *omni.ExtensionsConfiguration

			extensionsConfig, err = safe.StateGetByID[*omni.ExtensionsConfiguration](ctx, cli.Omni().State(), clusterMachine.Metadata().ID())
			if err != nil && !state.IsNotFoundError(err) {
				require.NoError(t, err)
			}

			updateSpec := func(res *omni.ExtensionsConfiguration) error {
				res.Metadata().Labels().Set(omni.LabelCluster, cluster)
				res.Metadata().Labels().Set(omni.LabelClusterMachine, clusterMachine.Metadata().ID())

				res.TypedSpec().Value.Extensions = extensions

				return nil
			}

			if extensionsConfig == nil {
				extensionsConfig = omni.NewExtensionsConfiguration(resources.DefaultNamespace, clusterMachine.Metadata().ID())

				require.NoError(t, updateSpec(extensionsConfig))

				require.NoError(t, cli.Omni().State().Create(ctx, extensionsConfig))

				continue
			}

			_, err = safe.StateUpdateWithConflicts[*omni.ExtensionsConfiguration](ctx, cli.Omni().State(), extensionsConfig.Metadata(), updateSpec)
			require.NoError(t, err)
		}
	}
}
