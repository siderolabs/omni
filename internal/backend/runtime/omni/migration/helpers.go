// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package migration

import (
	"context"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
)

func reconcileConfigInputs(ctx context.Context, st state.State, item *omni.ClusterMachine, withGenOptions, withTalosVersion bool) error {
	config := omni.NewClusterMachineConfig(resources.DefaultNamespace, item.Metadata().ID())

	_, err := st.Get(ctx, config.Metadata())
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil
		}

		return err
	}

	// update input versions on the cluster machine config to avoid its reconciliation
	clusterName, ok := item.Metadata().Labels().Get(omni.LabelCluster)
	if !ok {
		return nil
	}

	res := []resource.Resource{
		omni.NewClusterSecrets(resources.DefaultNamespace, clusterName),
		item,
		omni.NewLoadBalancerConfig(resources.DefaultNamespace, clusterName),
		omni.NewCluster(resources.DefaultNamespace, clusterName),
		omni.NewClusterMachineConfigPatches(resources.DefaultNamespace, item.Metadata().ID()),
	}

	if withGenOptions {
		res = append(res, omni.NewMachineConfigGenOptions(resources.DefaultNamespace, item.Metadata().ID()))
	}

	if withTalosVersion {
		res = append(res, omni.NewClusterMachineTalosVersion(resources.DefaultNamespace, item.Metadata().ID()))
	}

	inputs := make([]resource.Resource, 0, len(res))

	for _, res := range res {
		res, err = st.Get(ctx, res.Metadata())
		if err != nil {
			if state.IsNotFoundError(err) {
				return nil
			}

			return err
		}

		inputs = append(inputs, res)
	}

	_, err = safe.StateUpdateWithConflicts(ctx, st, config.Metadata(), func(machineConfig *omni.ClusterMachineConfig) error {
		helpers.UpdateInputsVersions(machineConfig, inputs...)

		machineConfig.TypedSpec().Value.ClusterMachineVersion = item.Metadata().Version().String()

		return nil
	}, state.WithUpdateOwner(omnictrl.ClusterMachineConfigControllerName), state.WithExpectedPhaseAny())

	return err
}
