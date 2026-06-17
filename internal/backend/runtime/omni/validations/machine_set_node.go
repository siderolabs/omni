// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package validations

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/validated"
)

// machineSetNodeValidationOptions returns the validation options for the machine set node resource.
//
//nolint:gocognit,gocyclo,cyclop,maintidx
func machineSetNodeValidationOptions(st state.State) []validated.StateOption {
	getMachineSet := func(ctx context.Context, res *omni.MachineSetNode) (*omni.MachineSet, string, error) {
		// relation label validations are done at relationLabelsValidationOptions
		machineSetName, _ := res.Metadata().Labels().Get(omni.LabelMachineSet)

		machineSet, err := safe.ReaderGet[*omni.MachineSet](ctx, st, omni.NewMachineSet(machineSetName).Metadata())
		if err != nil {
			if state.IsNotFoundError(err) {
				return nil, machineSetName, nil
			}

			return nil, machineSetName, err
		}

		return machineSet, machineSetName, nil
	}

	getCluster := func(ctx context.Context, res *omni.MachineSetNode) (*omni.Cluster, string, error) {
		// relation label validations are done at relationLabelsValidationOptions
		clusterName, _ := res.Metadata().Labels().Get(omni.LabelCluster)

		cluster, err := safe.ReaderGet[*omni.Cluster](ctx, st, omni.NewCluster(clusterName).Metadata())
		if err != nil {
			if state.IsNotFoundError(err) {
				return nil, clusterName, nil
			}

			return nil, clusterName, err
		}

		return cluster, clusterName, nil
	}

	validateTalosVersion := func(ctx context.Context, res *omni.MachineSetNode, cluster *omni.Cluster) error {
		machineStatus, err := safe.ReaderGetByID[*omni.MachineStatus](ctx, st, res.Metadata().ID())
		if err != nil {
			if state.IsNotFoundError(err) {
				return nil
			}

			return err
		}

		// ignore version check if it's not possible to parse machine Talos version
		if _, parseErr := semver.Parse(strings.TrimLeft(machineStatus.TypedSpec().Value.TalosVersion, "v")); parseErr != nil {
			return nil //nolint:nilerr
		}

		clusterTalosVersion, err := semver.Parse(cluster.TypedSpec().Value.TalosVersion)
		if err != nil {
			return err
		}

		ok, _, reason := omni.MachineCompatibleWithCluster(machineStatus, clusterTalosVersion)
		if !ok {
			return errors.New(reason)
		}

		return nil
	}

	// if the machine is created by the MachineSetNode controller
	// it means that the machine set is using a machine class and all MachineSetNodes are programmatically created
	// the MachineSetNode can still be changed by the user, but the creation should fail, unless the MachineSetNode
	// is re-created for an existing running ClusterMachine
	// this is to allow canceling destroy
	validateAutoMachineAllocation := func(ctx context.Context, st state.State, res *omni.MachineSetNode, machineSet *omni.MachineSet) error {
		clusterMachine, err := safe.ReaderGetByID[*omni.ClusterMachine](ctx, st, res.Metadata().ID())
		if err != nil && !state.IsNotFoundError(err) {
			return err
		}

		_, ok := res.Metadata().Labels().Get(omni.LabelManagedByMachineSetNodeController)
		// allow when cluster machine exists and running and the MachineSetNode has correct labels
		if clusterMachine != nil && ok && clusterMachine.Metadata().Phase() == resource.PhaseRunning {
			return nil
		}

		return fmt.Errorf("adding machine set node to the machine set %q is not allowed: the machine set is using automated machine allocation", machineSet.Metadata().ID())
	}

	return []validated.StateOption{
		validated.WithCreateValidations(validated.NewCreateValidationForType(func(ctx context.Context, res *omni.MachineSetNode, _ ...state.CreateOption) error {
			machineSet, machineSetName, err := getMachineSet(ctx, res)
			if err != nil {
				return err
			}

			if machineSet == nil {
				return fmt.Errorf("adding machine set node to the machine set %q is not allowed: the machine set does not exist", machineSetName)
			}

			if machineSet.Metadata().Phase() == resource.PhaseTearingDown {
				return fmt.Errorf("adding machine set node to the machine set %q is not allowed: the machine set is tearing down", machineSet.Metadata().ID())
			}

			if omni.GetMachineAllocation(machineSet) != nil {
				if err = validateAutoMachineAllocation(ctx, st, res, machineSet); err != nil {
					return err
				}
			}

			cluster, clusterName, err := getCluster(ctx, res)
			if err != nil {
				return err
			}

			if cluster == nil {
				return fmt.Errorf("adding machine set node to the machine set %q is not allowed: the cluster %q does not exist", machineSet.Metadata().ID(), clusterName)
			}

			if cluster.Metadata().Phase() == resource.PhaseRunning {
				_, importIsInProgress := cluster.Metadata().Annotations().Get(omni.ClusterImportIsInProgress)

				_, locked := cluster.Metadata().Annotations().Get(omni.ClusterLocked)
				if locked && !importIsInProgress {
					return fmt.Errorf("adding machine set node to the machine set %q is not allowed: the cluster %q is locked", machineSet.Metadata().ID(), cluster.Metadata().ID())
				}
			}

			if cluster.Metadata().Phase() == resource.PhaseTearingDown {
				return fmt.Errorf("adding machine set node to the machine set %q is not allowed: the cluster %q is tearing down", machineSet.Metadata().ID(), cluster.Metadata().ID())
			}

			if err = validateTalosVersion(ctx, res, cluster); err != nil {
				return err
			}

			return validateNotControlplane(machineSet, res)
		})),
		validated.WithUpdateValidations(validated.NewUpdateValidationForType(func(ctx context.Context, res *omni.MachineSetNode, newRes *omni.MachineSetNode, _ ...state.UpdateOption) error {
			if newRes.Metadata().Phase() == resource.PhaseTearingDown {
				// tearing down validations are done at destroy validations
				return nil
			}

			machineSet, machineSetName, err := getMachineSet(ctx, res)
			if err != nil {
				return err
			}

			if machineSet == nil {
				return fmt.Errorf("updating machine set node on the machine set %q is not allowed: the machine set does not exist", machineSetName)
			}

			if machineSet.Metadata().Phase() == resource.PhaseTearingDown {
				return fmt.Errorf("updating machine set node on the machine set %q is not allowed: the machine set is tearing down", machineSet.Metadata().ID())
			}

			cluster, clusterName, err := getCluster(ctx, res)
			if err != nil {
				return err
			}

			if cluster == nil {
				return fmt.Errorf("updating machine set node on the machine set %q is not allowed: the cluster %q does not exist", machineSet.Metadata().ID(), clusterName)
			}

			if cluster.Metadata().Phase() == resource.PhaseRunning {
				_, importIsInProgress := cluster.Metadata().Annotations().Get(omni.ClusterImportIsInProgress)

				_, locked := cluster.Metadata().Annotations().Get(omni.ClusterLocked)
				if locked && !importIsInProgress {
					return fmt.Errorf("updating machine set node on the machine set %q is not allowed: the cluster %q is locked", machineSet.Metadata().ID(), cluster.Metadata().ID())
				}
			}

			if cluster.Metadata().Phase() == resource.PhaseTearingDown {
				return fmt.Errorf("updating machine set node on the machine set %q is not allowed: the cluster %q is tearing down", machineSet.Metadata().ID(), cluster.Metadata().ID())
			}

			newMachineSet, ok := newRes.Metadata().Labels().Get(omni.LabelMachineSet)
			if !ok {
				return fmt.Errorf("missing machine set label")
			}

			if newMachineSet != machineSetName {
				if err = validateTalosVersion(ctx, res, cluster); err != nil {
					return err
				}
			}

			return validateNotControlplane(machineSet, newRes)
		})),
		validated.WithDestroyValidations(validated.NewDestroyValidationForType(func(ctx context.Context, _ resource.Pointer, res *omni.MachineSetNode, _ ...state.DestroyOption) error {
			machineSet, machineSetName, err := getMachineSet(ctx, res)
			if err != nil {
				return err
			}

			cluster, clusterName, err := getCluster(ctx, res)
			if err != nil {
				return err
			}

			// machine set doesn't exist or being destroyed, do not block the deletion
			if machineSet == nil || machineSet.Metadata().Phase() == resource.PhaseTearingDown {
				return nil
			}

			// cluster doesn't exist or being destroyed, do not block the deletion
			if cluster == nil || cluster.Metadata().Phase() == resource.PhaseTearingDown {
				return nil
			}

			if _, locked := res.Metadata().Annotations().Get(omni.MachineLocked); locked {
				return fmt.Errorf("removing machine set node from the machine set %q is not allowed: machine set node is locked", machineSetName)
			}

			if _, locked := cluster.Metadata().Annotations().Get(omni.ClusterLocked); locked {
				return fmt.Errorf("removing machine set node from the machine set %q is not allowed: the cluster %q is locked", machineSetName, clusterName)
			}

			return nil
		})),
	}
}

func validateNotControlplane(machineSet *omni.MachineSet, res *omni.MachineSetNode) error {
	if _, locked := res.Metadata().Annotations().Get(omni.MachineLocked); !locked {
		return nil
	}

	if machineSet == nil {
		return nil
	}

	if _, cp := machineSet.Metadata().Labels().Get(omni.LabelControlPlaneRole); cp {
		return errors.New("locking controlplanes is not allowed")
	}

	return nil
}
