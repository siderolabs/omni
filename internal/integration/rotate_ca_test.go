// Copyright (c) 2026 Sidero Labs, Inc.
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

	clusterName := "integration-rotate-ca"

	// Create a cluster to make sure that we have Talos installed on a machine
	t.Run(
		"ClusterShouldBeCreated",
		CreateCluster(t.Context(), options, ClusterOptions{
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
		assertCARotated(ctx, t, omniState, clusterName, omni.NewRotateTalosCA(clusterName))
	})

	runTests(t, AssertBlockClusterAndTalosAPIAndKubernetesShouldBeReady(t.Context(), options, clusterName, options.MachineOptions.TalosVersion, options.MachineOptions.KubernetesVersion))

	t.Run("KubernetesCAShouldBeRotated", func(t *testing.T) {
		assertCARotated(ctx, t, omniState, clusterName, omni.NewRotateKubernetesCA(clusterName))
	})

	runTests(t, AssertBlockClusterAndTalosAPIAndKubernetesShouldBeReady(t.Context(), options, clusterName, options.MachineOptions.TalosVersion, options.MachineOptions.KubernetesVersion))

	t.Run("ClusterShouldBeDestroyed", AssertDestroyCluster(t.Context(), options.omniClient.Omni().State(), clusterName, false, false))
}

func assertCARotated(ctx context.Context, t *testing.T, omniState state.State, clusterName string, rotation resource.Resource) {
	require.NoError(t, omniState.Create(ctx, rotation))

	// assert rotation started
	require.NoError(t, waitForRotationPhase(ctx, omniState, clusterName, func(phase specs.SecretRotationSpec_Phase) bool {
		return phase != specs.SecretRotationSpec_OK
	}))

	// assert rotation completed
	require.NoError(t, waitForRotationPhase(ctx, omniState, clusterName, func(phase specs.SecretRotationSpec_Phase) bool {
		return phase == specs.SecretRotationSpec_OK
	}))
}

func waitForRotationPhase(ctx context.Context, omniState state.State, clusterName string, matches func(specs.SecretRotationSpec_Phase) bool) error {
	_, err := safe.StateWatchFor[*omni.ClusterSecretsRotationStatus](ctx, omniState, omni.NewClusterSecretsRotationStatus(clusterName).Metadata(), func(cond *state.WatchForCondition) error {
		cond.Condition = func(res resource.Resource) (bool, error) {
			resTyped, ok := res.(*omni.ClusterSecretsRotationStatus)
			if !ok {
				return false, fmt.Errorf("unexpected resource type: %T", res)
			}

			return matches(resTyped.TypedSpec().Value.Phase), nil
		}

		return nil
	})

	return err
}
