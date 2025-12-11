// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build integration

package integration_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

func testRotateCA(t *testing.T, options *TestOptions) {
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Minute)
	defer cancel()

	options.claimMachines(t, 5)
	omniClient := options.omniClient

	clusterName := "integration-rotate-ca"

	// Create a cluster to make sure that we have Talos installed on a machine
	t.Run(
		"ClusterShouldBeCreated",
		CreateCluster(t.Context(), omniClient, ClusterOptions{
			Name:          clusterName,
			ControlPlanes: 3,
			Workers:       2,

			MachineOptions: options.MachineOptions,
			ScalingTimeout: options.ScalingTimeout,

			SkipExtensionCheckOnCreate: options.SkipExtensionsCheckOnCreate,
		}),
	)

	assertClusterAndAPIReady(t, clusterName, options)

	omniState := options.omniClient.Omni().State()

	t.Run("TalosCAShouldBeRotated", func(t *testing.T) {
		rotateTalosCA := omni.NewRotateTalosCA(clusterName)
		require.NoError(t, omniState.Create(ctx, rotateTalosCA))

		// assert rotation started
		_, err := safe.StateWatchFor[*omni.ClusterSecretsRotationStatus](ctx, omniState, omni.NewClusterSecretsRotationStatus(clusterName).Metadata(), func(cond *state.WatchForCondition) error {
			cond.Condition = func(res resource.Resource) (bool, error) {
				resTyped, ok := res.(*omni.ClusterSecretsRotationStatus)
				if !ok {
					return false, fmt.Errorf("unexpected resource type: %T", res)
				}

				if resTyped.TypedSpec().Value.Phase != specs.ClusterSecretsRotationStatusSpec_OK {
					return true, nil
				}

				return false, nil
			}

			return nil
		})
		require.NoError(t, err)

		// assert rotation completed
		_, err = safe.StateWatchFor[*omni.ClusterSecretsRotationStatus](ctx, omniState, omni.NewClusterSecretsRotationStatus(clusterName).Metadata(), func(cond *state.WatchForCondition) error {
			cond.Condition = func(res resource.Resource) (bool, error) {
				resTyped, ok := res.(*omni.ClusterSecretsRotationStatus)
				if !ok {
					return false, fmt.Errorf("unexpected resource type: %T", res)
				}

				if resTyped.TypedSpec().Value.Phase == specs.ClusterSecretsRotationStatusSpec_OK {
					return true, nil
				}

				return false, nil
			}

			return nil
		})
		require.NoError(t, err)
	})

	runTests(t, AssertBlockClusterAndTalosAPIAndKubernetesShouldBeReady(t.Context(), omniClient, clusterName, options.MachineOptions.TalosVersion, options.MachineOptions.KubernetesVersion))

	t.Run("ClusterShouldBeDestroyed", AssertDestroyCluster(t.Context(), options.omniClient.Omni().State(), clusterName, false, false))
}
