// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build integration

package integration_test

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/go-retry/retry"
	"github.com/siderolabs/image-factory/pkg/constants"
	talosclient "github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/extensions"
)

// HelloWorldServiceExtensionName is the name of the sample hello world extension used for testing.
const HelloWorldServiceExtensionName = extensions.OfficialPrefix + "hello-world-service"

// AssertExtensionsArePresent asserts that the given extensions are all present on all machines of the given cluster.
func AssertExtensionsArePresent(ctx context.Context, options *TestOptions, cluster string, extensions []string) TestFunc {
	return func(t *testing.T) {
		clusterMachineList, err := safe.StateListAll[*omni.ClusterMachine](ctx, options.omniClient.Omni().State(), state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, cluster)))
		require.NoError(t, err)

		machineIDs := make([]resource.ID, 0, clusterMachineList.Len())

		clusterMachineList.ForEach(func(clusterMachine *omni.ClusterMachine) {
			machineIDs = append(machineIDs, clusterMachine.Metadata().ID())
		})

		checkExtensionsWithRetries(ctx, t, options, extensions, machineIDs)
	}
}

func checkExtensionsWithRetries(ctx context.Context, t *testing.T, options *TestOptions, extensions []string, machineIDs []resource.ID) {
	for _, machineID := range machineIDs {
		numErrs := 0

		nodeCtx := talosclient.WithNode(ctx, machineID)
		talosClient := getTalosClient(nodeCtx, t, options)

		err := retry.Constant(3*time.Minute, retry.WithUnits(time.Second), retry.WithAttemptTimeout(3*time.Second)).RetryWithContext(ctx, func(ctx context.Context) error {
			if err := checkExtensions(nodeCtx, talosClient, extensions); err != nil {
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
func checkExtensions(ctx context.Context, talosClient *talosclient.Client, extensions []string) error {
	collectedExtensions, err := fetchExtensions(ctx, talosClient)
	if err != nil {
		return err
	}

	pos := 0
	for _, ext := range extensions {
		i := slices.Index(collectedExtensions[pos:], ext)
		if i < 0 {
			return fmt.Errorf("extensions/order mismatch: expected %q to be a subsequence of %q", extensions, collectedExtensions)
		}
		pos += i + 1
	}

	return nil
}

func fetchExtensions(ctx context.Context, talosClient *talosclient.Client) ([]string, error) {
	list, err := safe.StateListAll[*runtime.ExtensionStatus](ctx, talosClient.COSI)
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
				extensionsConfig = omni.NewExtensionsConfiguration(clusterMachine.Metadata().ID())

				require.NoError(t, updateSpec(extensionsConfig))

				require.NoError(t, cli.Omni().State().Create(ctx, extensionsConfig))

				continue
			}

			_, err = safe.StateUpdateWithConflicts[*omni.ExtensionsConfiguration](ctx, cli.Omni().State(), extensionsConfig.Metadata(), updateSpec)
			require.NoError(t, err)
		}
	}
}
