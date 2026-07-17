// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package validations

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/etcdbackup/store"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/validated"
)

// MaxBootstrapSnapshotLength caps the length of MachineSet.bootstrap_spec.snapshot in bytes.
// Anchored on Linux PATH_MAX, which is the most permissive of the supported backup stores'
// path limits, so any value that could legitimately reach either backend fits.
const MaxBootstrapSnapshotLength = 4096

// machineSetValidationOptions returns the validation options for the machine set resource.
//
//nolint:gocognit,gocyclo,cyclop,maintidx
func machineSetValidationOptions(st state.State, etcdBackupStoreFactory store.Factory) []validated.StateOption {
	validate := func(ctx context.Context, oldRes *omni.MachineSet, res *omni.MachineSet) error {
		// relation label validations are done at relationLabelsValidationOptions
		clusterName, _ := res.Metadata().Labels().Get(omni.LabelCluster)

		cluster, err := safe.StateGetByID[*omni.Cluster](ctx, st, clusterName)
		if err != nil && !state.IsNotFoundError(err) {
			return err
		}

		// if cluster doesn't exist and machine set is being destroyed, skip the rest of the validations
		if cluster == nil {
			if res.Metadata().Phase() == resource.PhaseTearingDown {
				return nil
			}

			return fmt.Errorf("the cluster %q does not exist", clusterName)
		}

		_, locked := cluster.Metadata().Annotations().Get(omni.ClusterLocked)

		_, importIsInProgress := cluster.Metadata().Annotations().Get(omni.ClusterImportIsInProgress)
		if oldRes == nil {
			if cluster.Metadata().Phase() == resource.PhaseTearingDown {
				return fmt.Errorf("the cluster %q is tearing down", clusterName)
			}

			if locked && !importIsInProgress {
				return fmt.Errorf("adding machine set %q to the cluster %q is not allowed: the cluster is locked", res.Metadata().ID(), clusterName)
			}
		}

		if locked && !importIsInProgress && res.Metadata().Phase() == resource.PhaseRunning {
			return fmt.Errorf("updating machine set %q on the cluster %q is not allowed: the cluster is locked", res.Metadata().ID(), clusterName)
		}

		_, isControlPlane := res.Metadata().Labels().Get(omni.LabelControlPlaneRole)
		_, isWorker := res.Metadata().Labels().Get(omni.LabelWorkerRole)

		if !isControlPlane && !isWorker {
			return fmt.Errorf("machine set must have either %q or %q label", omni.LabelControlPlaneRole, omni.LabelWorkerRole)
		}

		if isControlPlane && oldRes == nil { // creating a new control plane machine set
			bootstrapped, err := clusterBootstrapped(ctx, st, clusterName)
			if err != nil {
				return err
			}

			if bootstrapped {
				return errors.New("adding control plane machine set to an already bootstrapped cluster is not allowed")
			}
		}

		if err := validateBootstrapSpec(ctx, st, etcdBackupStoreFactory, oldRes, res); err != nil {
			return err
		}

		allocationConfig := omni.GetMachineAllocation(res)

		if allocationConfig != nil {
			if allocationConfig.Name == "" {
				return errors.New("machine allocation source name is not set")
			}

			if allocationConfig.MachineCount != 0 && allocationConfig.AllocationType != specs.MachineSetSpec_MachineAllocation_Static {
				return errors.New("machine count can be set only if static allocation type is used")
			}

			var oldAllocationConfig *specs.MachineSetSpec_MachineAllocation

			if oldRes != nil {
				oldAllocationConfig = omni.GetMachineAllocation(oldRes)
			}

			// if change machine class, verify the specified class name exists.
			changed := oldRes == nil || oldAllocationConfig != nil && oldAllocationConfig.Name != allocationConfig.Name
			if changed {
				mc, err := safe.ReaderGetByID[*omni.MachineClass](ctx, st, allocationConfig.Name)
				if err != nil {
					if state.IsNotFoundError(err) {
						return fmt.Errorf("machine class with name %q doesn't exist", allocationConfig.Name)
					}

					return err
				}

				if mc.TypedSpec().Value.AutoProvision != nil && allocationConfig.AllocationType == specs.MachineSetSpec_MachineAllocation_Unlimited {
					return fmt.Errorf("machine class %q is using autoprovision, so unlimited machine set allocation is not supported", allocationConfig.Name)
				}
			}
		}

		if oldRes != nil {
			// ensure that the machine class type doesn't change from manually selected machines to the machine class
			oldAllocationConfig := omni.GetMachineAllocation(oldRes)
			newAllocationConfig := omni.GetMachineAllocation(res)

			mgmtModeSwitchedToMachineClass := oldAllocationConfig == nil && newAllocationConfig != nil
			mgmtModeSwitchedToManual := oldAllocationConfig != nil && newAllocationConfig == nil
			mgmtModeChanged := mgmtModeSwitchedToMachineClass || mgmtModeSwitchedToManual

			if mgmtModeChanged {
				machineSetNodeList, err := safe.StateListAll[*omni.MachineSetNode](ctx, st, state.WithLabelQuery(resource.LabelEqual(omni.LabelMachineSet, res.Metadata().ID())))
				if err != nil {
					return fmt.Errorf("error getting machine set nodes: %w", err)
				}

				// block management mode change only if there are nodes in the machine set
				if machineSetNodeList.Len() > 0 {
					switch {
					case mgmtModeSwitchedToMachineClass:
						return errors.New("machine set is not empty and is using manual nodes management, updating to machine class mode is not allowed")
					case mgmtModeSwitchedToManual:
						return errors.New("machine set is not empty and is using machine class based node management, updating to manual mode is not allowed")
					}
				}
			}

			return nil
		}

		// id validations
		clusterPrefix := clusterName + "-"

		if !strings.HasPrefix(res.Metadata().ID(), clusterPrefix) {
			return fmt.Errorf("machine set of cluster %q ID must have %q as prefix", clusterName, clusterPrefix)
		}

		cpID := omni.ControlPlanesResourceID(clusterName)

		if isControlPlane {
			if res.Metadata().ID() == cpID {
				return nil
			}

			return fmt.Errorf("control plane machine set must have ID %q", cpID)
		}

		if res.Metadata().ID() == cpID {
			return fmt.Errorf("worker machine set must not have ID %q", cpID)
		}

		return nil
	}

	validateDelete := func(ctx context.Context, res *omni.MachineSet) error {
		if res == nil {
			return nil
		}

		// relation label validations are done at relationLabelsValidationOptions
		clusterName, _ := res.Metadata().Labels().Get(omni.LabelCluster)

		cluster, err := safe.StateGetByID[*omni.Cluster](ctx, st, clusterName)
		if err != nil && !state.IsNotFoundError(err) {
			return err
		}

		if cluster == nil {
			return nil
		}

		if _, locked := cluster.Metadata().Annotations().Get(omni.ClusterLocked); locked && cluster.Metadata().Phase() == resource.PhaseRunning {
			return fmt.Errorf("removing machine set %q from the cluster %q is not allowed: the cluster is locked", res.Metadata().ID(), clusterName)
		}

		return nil
	}

	return []validated.StateOption{
		validated.WithCreateValidations(validated.NewCreateValidationForType(func(ctx context.Context, res *omni.MachineSet, _ ...state.CreateOption) error {
			return validate(ctx, nil, res)
		})),
		validated.WithUpdateValidations(validated.NewUpdateValidationForType(func(ctx context.Context, oldRes *omni.MachineSet, newRes *omni.MachineSet, _ ...state.UpdateOption) error {
			return validate(ctx, oldRes, newRes)
		})),
		validated.WithDestroyValidations(validated.NewDestroyValidationForType(func(ctx context.Context, _ resource.Pointer, existingRes *omni.MachineSet, option ...state.DestroyOption) error {
			return validateDelete(ctx, existingRes)
		})),
	}
}

func validateBootstrapSpec(ctx context.Context, st state.State, etcdBackupStoreFactory store.Factory, oldres, res *omni.MachineSet) error {
	bootstrapSpec := res.TypedSpec().Value.GetBootstrapSpec()
	_, isControlPlane := res.Metadata().Labels().Get(omni.LabelControlPlaneRole)

	if !isControlPlane && bootstrapSpec != nil {
		return errors.New("bootstrap spec is not allowed for worker machine sets")
	}

	if oldres != nil { // this is an update
		oldBootstrapSpec := oldres.TypedSpec().Value.GetBootstrapSpec()

		if bootstrapSpec == nil && oldBootstrapSpec != nil {
			// the bootstrap spec has no effect after the cluster is bootstrapped, so it is safe to remove it at that point.
			// removing it earlier would silently turn a restore from a backup into a fresh cluster bootstrap.
			clusterName, _ := res.Metadata().Labels().Get(omni.LabelCluster)

			bootstrapped, err := clusterBootstrapped(ctx, st, clusterName)
			if err != nil {
				return err
			}

			if !bootstrapped {
				return errors.New("bootstrap spec can only be removed after the cluster is bootstrapped")
			}

			return nil
		}

		if !bootstrapSpec.EqualVT(oldBootstrapSpec) {
			return errors.New("bootstrap spec is immutable after creation")
		}

		// short-circuit if the bootstrap spec is not changed on update - it was already validated on creation
		return nil
	}

	if bootstrapSpec == nil {
		return nil
	}

	if err := validateBootstrapSnapshot(bootstrapSpec.GetSnapshot()); err != nil {
		return err
	}

	clusterUUIDs, err := safe.StateListAll[*omni.ClusterUUID](ctx, st, state.WithLabelQuery(resource.LabelEqual(omni.LabelClusterUUID, bootstrapSpec.GetClusterUuid())))
	if err != nil {
		return fmt.Errorf("error getting cluster UUIDs: %w", err)
	}

	if clusterUUIDs.Len() == 0 {
		return fmt.Errorf("invalid cluster UUID %q", bootstrapSpec.GetClusterUuid())
	}

	if clusterUUIDs.Len() > 1 {
		return fmt.Errorf("inconsistent state on cluster UUID %q", bootstrapSpec.GetClusterUuid())
	}

	cluster := clusterUUIDs.Get(0).Metadata().ID()

	backupData, err := safe.ReaderGetByID[*omni.BackupData](ctx, st, cluster)
	if err != nil {
		return fmt.Errorf("error getting backup data: %w", err)
	}

	backupStore, err := etcdBackupStoreFactory.GetStore()
	if err != nil {
		return fmt.Errorf("error getting etcd backup store: %w", err)
	}

	data, readCloser, err := backupStore.Download(ctx, backupData.TypedSpec().Value.EncryptionKey, bootstrapSpec.ClusterUuid, bootstrapSpec.Snapshot)
	if err != nil {
		return fmt.Errorf("failed to get backup: %w", err)
	}

	readCloser.Close() //nolint:errcheck

	if data.AESCBCEncryptionSecret != backupData.TypedSpec().Value.AesCbcEncryptionSecret {
		return errors.New("aes cbc encryption secret mismatch")
	}

	if data.SecretboxEncryptionSecret != backupData.TypedSpec().Value.SecretboxEncryptionSecret {
		return errors.New("secretbox encryption secret mismatch")
	}

	return nil
}

func clusterBootstrapped(ctx context.Context, st state.State, clusterName resource.ID) (bool, error) {
	bootstrapStatus, err := safe.StateGetByID[*omni.ClusterBootstrapStatus](ctx, st, clusterName)
	if err != nil && !state.IsNotFoundError(err) {
		return false, fmt.Errorf("error getting cluster bootstrap status: %w", err)
	}

	return bootstrapStatus != nil && bootstrapStatus.TypedSpec().Value.GetBootstrapped(), nil
}

func validateBootstrapSnapshot(snapshot string) error {
	if snapshot == "" {
		return nil
	}

	if len(snapshot) > MaxBootstrapSnapshotLength {
		return fmt.Errorf("bootstrap snapshot path is too long: %d bytes (max %d)", len(snapshot), MaxBootstrapSnapshotLength)
	}

	if !fs.ValidPath(snapshot) {
		return errors.New("bootstrap snapshot must be a relative path without parent-directory references")
	}

	return nil
}
