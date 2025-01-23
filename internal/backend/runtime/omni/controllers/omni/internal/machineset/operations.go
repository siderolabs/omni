// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package machineset

import (
	"bytes"
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/set"
)

// Operation is a single operation which alters the machine set.
type Operation interface {
	Apply(ctx context.Context, r controller.ReaderWriter, logger *zap.Logger, rc *ReconciliationContext) error
}

// Create the cluster machine.
type Create struct {
	ID string
}

// Apply implements Operation interface.
func (c *Create) Apply(ctx context.Context, r controller.ReaderWriter, logger *zap.Logger, rc *ReconciliationContext) error {
	clusterMachine := omni.NewClusterMachine(resources.DefaultNamespace, c.ID)
	clusterMachineConfigPatches := omni.NewClusterMachineConfigPatches(resources.DefaultNamespace, c.ID)

	machineSet := rc.GetMachineSet()
	configPatches := rc.GetConfigPatches(c.ID)

	helpers.CopyLabels(machineSet, clusterMachineConfigPatches, omni.LabelCluster, omni.LabelWorkerRole, omni.LabelControlPlaneRole)
	helpers.CopyLabels(machineSet, clusterMachine, omni.LabelCluster, omni.LabelWorkerRole, omni.LabelControlPlaneRole)

	clusterMachine.Metadata().Labels().Set(omni.LabelMachineSet, machineSet.Metadata().ID())
	clusterMachineConfigPatches.Metadata().Labels().Set(omni.LabelMachineSet, machineSet.Metadata().ID())

	helpers.UpdateInputsVersions(clusterMachine, configPatches...)

	if err := setPatches(clusterMachineConfigPatches, configPatches); err != nil {
		return err
	}

	var err error

	clusterMachine.TypedSpec().Value.KubernetesVersion, err = rc.GetKubernetesVersion()
	if err != nil {
		return err
	}

	logger.Info("create cluster machine", zap.String("machine", c.ID))

	if err = r.Create(ctx, clusterMachine); err != nil {
		return err
	}

	return r.Create(ctx, clusterMachineConfigPatches)
}

// Teardown the cluster machine.
type Teardown struct {
	Quota *ChangeQuota
	ID    string
}

// Apply implements Operation interface.
func (d *Teardown) Apply(ctx context.Context, r controller.ReaderWriter, logger *zap.Logger, rc *ReconciliationContext) error {
	if !d.Quota.Use(opDelete) {
		logger.Info("teardown is waiting for quota", zap.String("machine", d.ID), zap.Strings("pending", set.Values(rc.GetTearingDownMachines())))

		return nil
	}

	logger.Info("teardown cluster machine", zap.String("machine", d.ID))

	if _, err := r.Teardown(ctx, omni.NewClusterMachine(resources.DefaultNamespace, d.ID).Metadata()); err != nil {
		return fmt.Errorf(
			"error tearing down machine %q in cluster %q: %w",
			d.ID,
			rc.GetClusterName(),
			err,
		)
	}

	return nil
}

// Update the configs of the machine by updating the ClusterMachineConfigPatches.
type Update struct {
	Quota *ChangeQuota
	ID    string
}

// Apply implements Operation interface.
func (u *Update) Apply(ctx context.Context, r controller.ReaderWriter, logger *zap.Logger, rc *ReconciliationContext) error {
	clusterMachine, ok := rc.GetClusterMachine(u.ID)
	if !ok {
		return fmt.Errorf("cluster machine with id %q doesn't exist", u.ID)
	}

	configPatches := rc.GetConfigPatches(u.ID)

	// nothing changed in the patch list, skip any updates
	if !helpers.UpdateInputsVersions(clusterMachine, configPatches...) {
		return nil
	}

	ignoreQuota := rc.GetUpdatingMachines().Contains(u.ID)

	// if updating we don't care about the quota
	if !ignoreQuota {
		if !u.Quota.Use(opUpdate) {
			logger.Info("update is waiting for quota", zap.String("machine", u.ID), zap.Strings("pending", set.Values(rc.GetUpdatingMachines())))

			return nil
		}
	}

	logger.Info("update cluster machine", zap.String("machine", u.ID), zap.Bool("ignore_quota", ignoreQuota))

	// update ClusterMachineConfigPatches resource with the list of matching patches for the machine
	err := safe.WriterModify(ctx, r, omni.NewClusterMachineConfigPatches(resources.DefaultNamespace, u.ID),
		func(clusterMachineConfigPatches *omni.ClusterMachineConfigPatches) error {
			return setPatches(clusterMachineConfigPatches, configPatches)
		},
	)
	if err != nil {
		return err
	}

	// finally update checksum for the incoming config patches in the ClusterMachine resource
	return safe.WriterModify(ctx, r, clusterMachine, func(res *omni.ClusterMachine) error {
		// don't update the ClusterMachine if it's still owned by another cluster
		currentClusterName, ok := res.Metadata().Labels().Get(omni.LabelCluster)
		if ok && currentClusterName != rc.GetClusterName() {
			return nil
		}

		helpers.CopyAllAnnotations(clusterMachine, res)

		if res.TypedSpec().Value.KubernetesVersion == "" {
			res.TypedSpec().Value.KubernetesVersion, err = rc.GetKubernetesVersion()
			if err != nil {
				return err
			}
		}

		return nil
	})
}

// Destroy cleans up the cluster machines without finalizers.
type Destroy struct {
	ID string
}

// Apply implements Operation interface.
func (d *Destroy) Apply(ctx context.Context, r controller.ReaderWriter, logger *zap.Logger, rc *ReconciliationContext) error {
	clusterMachine, ok := rc.GetClusterMachine(d.ID)
	if !ok {
		return fmt.Errorf("machine with id %q doesn't exist in the state", d.ID)
	}

	if clusterMachine.Metadata().Phase() == resource.PhaseRunning {
		return nil
	}

	if !clusterMachine.Metadata().Finalizers().Empty() {
		return nil
	}

	configPatches := omni.NewClusterMachineConfigPatches(clusterMachine.Metadata().Namespace(), clusterMachine.Metadata().ID())

	if err := r.Destroy(ctx, configPatches.Metadata()); err != nil && !state.IsNotFoundError(err) {
		return err
	}

	if err := r.Destroy(ctx, clusterMachine.Metadata()); err != nil && !state.IsNotFoundError(err) {
		return err
	}

	// release the Machine finalizer
	if err := r.RemoveFinalizer(
		ctx,
		omni.NewMachine(resources.DefaultNamespace, clusterMachine.Metadata().ID()).Metadata(),
		ControllerName,
	); err != nil && !state.IsNotFoundError(err) {
		return fmt.Errorf("error removing finalizer from machine %q: %w", clusterMachine.Metadata().ID(), err)
	}

	logger.Info("deleted the machine",
		zap.String("machine", clusterMachine.Metadata().ID()),
	)

	return nil
}

//nolint:staticcheck // we are ok with using the deprecated method here
func setPatches(clusterMachineConfigPatches *omni.ClusterMachineConfigPatches, patches []*omni.ConfigPatch) error {
	// clear the patch fields
	clusterMachineConfigPatches.TypedSpec().Value.Patches = nil
	clusterMachineConfigPatches.TypedSpec().Value.CompressedPatches = nil

	if specs.GetCompressionConfig().Enabled {
		return setPatchesCompress(clusterMachineConfigPatches, patches)
	}

	return setPatchesNoCompress(clusterMachineConfigPatches, patches)
}

//nolint:staticcheck // we are ok with using the deprecated method here
func setPatchesCompress(res *omni.ClusterMachineConfigPatches, patches []*omni.ConfigPatch) error {
	for _, patch := range patches {
		compressedSize := len(patch.TypedSpec().Value.CompressedData)

		if compressedSize == 0 { // patch is not compressed, compress and then append it
			if isEmptyPatch(patch) {
				continue
			}

			buffer, err := patch.TypedSpec().Value.GetUncompressedData()
			if err != nil {
				return err
			}

			if err = patch.TypedSpec().Value.SetUncompressedData(buffer.Data()); err != nil {
				return err
			}

			res.TypedSpec().Value.CompressedPatches = append(res.TypedSpec().Value.CompressedPatches, patch.TypedSpec().Value.CompressedData)

			buffer.Free() // we can already free the buffer, as we already read and compressed it

			continue
		}

		// patch is compressed, append it directly

		if compressedSize < 1024 { // this is a small patch, decompress to check if it's all whitespace
			if isEmptyPatch(patch) {
				continue
			}
		}

		// append the patch
		res.TypedSpec().Value.CompressedPatches = append(res.TypedSpec().Value.CompressedPatches, patch.TypedSpec().Value.CompressedData)
	}

	return nil
}

//nolint:staticcheck // we are ok with using the deprecated method here
func setPatchesNoCompress(res *omni.ClusterMachineConfigPatches, patches []*omni.ConfigPatch) error {
	for _, patch := range patches {
		compressedSize := len(patch.TypedSpec().Value.CompressedData)

		if compressedSize == 0 { // not compressed, append the patch
			if isEmptyPatch(patch) {
				continue
			}

			res.TypedSpec().Value.Patches = append(res.TypedSpec().Value.Patches, patch.TypedSpec().Value.Data)

			continue
		}

		// compressed, decompress and append the patch

		buffer, err := patch.TypedSpec().Value.GetUncompressedData()
		if err != nil {
			return err
		}

		res.TypedSpec().Value.Patches = append(res.TypedSpec().Value.Patches, string(buffer.Data()))

		buffer.Free() // we can already free the buffer, as we converted its bytes to a string
	}

	return nil
}

func isEmptyPatch(patch *omni.ConfigPatch) bool {
	buffer, err := patch.TypedSpec().Value.GetUncompressedData()
	if err != nil {
		return false
	}

	defer buffer.Free()

	return len(bytes.TrimSpace(buffer.Data())) == 0
}
