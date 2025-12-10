// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package machineconfig

import (
	"context"
	"fmt"
	"strings"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xerrors"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/kernelargs"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/talos"
)

// ReconciliationContext describes all related data for one reconciliation call of the machine config status controller.
type ReconciliationContext struct {
	machineStatus         *omni.MachineStatus
	machineConfig         *omni.ClusterMachineConfig
	machineConfigStatus   *omni.ClusterMachineConfigStatus
	machineStatusSnapshot *omni.MachineStatusSnapshot
	installImage          *specs.MachineConfigGenOptionsSpec_InstallImage

	lastConfigError        string
	shouldUpgrade          bool
	compareFullSchematicID bool
}

func checkClusterReady(ctx context.Context, r controller.Reader, machineConfig *omni.ClusterMachineConfig) error {
	clusterName, ok := machineConfig.Metadata().Labels().Get(omni.LabelCluster)
	if !ok {
		return nil
	}

	cluster, err := safe.ReaderGetByID[*omni.Cluster](ctx, r, clusterName)
	if err != nil {
		if state.IsNotFoundError(err) {
			return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("cluster doesn't exist")
		}

		return err
	}

	if _, locked := cluster.Metadata().Annotations().Get(omni.ClusterLocked); locked && cluster.Metadata().Phase() == resource.PhaseRunning {
		return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("reconciling cluster machine config is not allowed: the cluster %q is locked", clusterName)
	}

	return nil
}

func checkMachineStatus(ctx context.Context, r controller.Reader, machineStatus *omni.MachineStatus) error {
	if !machineStatus.TypedSpec().Value.Connected {
		return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("'%s' machine is not connected", machineStatus.Metadata().ID())
	}

	if machineStatus.TypedSpec().Value.Schematic == nil {
		return xerrors.NewTagged[qtransform.SkipReconcileTag](fmt.Errorf("machine status '%s' does not have schematic information", machineStatus.Metadata().ID()))
	}

	if machineStatus.TypedSpec().Value.Schematic.InAgentMode {
		return xerrors.NewTagged[qtransform.SkipReconcileTag](fmt.Errorf("machine status '%s' schematic is in agent mode", machineStatus.Metadata().ID()))
	}

	// if the machine is managed by a static infra provider, we need to ensure that the infra machine is ready to use
	if _, isManagedByStaticInfraProvider := machineStatus.Metadata().Labels().Get(omni.LabelIsManagedByStaticInfraProvider); isManagedByStaticInfraProvider {
		var infraMachineStatus *infra.MachineStatus

		infraMachineStatus, err := safe.ReaderGetByID[*infra.MachineStatus](ctx, r, machineStatus.Metadata().ID())
		if err != nil {
			if state.IsNotFoundError(err) {
				return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("machine is managed by a static infra provider but the infra machine status is not found: %w", err)
			}

			return fmt.Errorf("failed to get infra machine status '%s': %w", machineStatus.Metadata().ID(), err)
		}

		if !infraMachineStatus.TypedSpec().Value.ReadyToUse {
			return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("machine is managed by static infra provider but is not ready to use")
		}
	}

	return nil
}

func (rc *ReconciliationContext) ID() string {
	return rc.machineConfig.Metadata().ID()
}

// SchematicEqual compares the schematic using the complex condition.
func (rc *ReconciliationContext) SchematicEqual(schematicInfo talos.SchematicInfo, expectedSchematic string) bool {
	if rc.compareFullSchematicID {
		return schematicInfo.FullID == expectedSchematic
	}

	return schematicInfo.Equal(expectedSchematic)
}

// BuildReconciliationContext is the COSI reader dependent method to build the reconciliation context.
func BuildReconciliationContext(ctx context.Context, r controller.Reader,
	machineConfig *omni.ClusterMachineConfig, machineConfigStatus *omni.ClusterMachineConfigStatus,
) (*ReconciliationContext, error) {
	if machineConfig.TypedSpec().Value.GenerationError != "" {
		return &ReconciliationContext{
			lastConfigError: machineConfig.TypedSpec().Value.GenerationError,
		}, nil
	}

	if err := checkClusterReady(ctx, r, machineConfig); err != nil {
		return nil, err
	}

	statusSnapshot, err := safe.ReaderGet[*omni.MachineStatusSnapshot](ctx, r,
		omni.NewMachineStatusSnapshot(resources.DefaultNamespace, machineConfig.Metadata().ID()).Metadata(),
	)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, xerrors.NewTaggedf[qtransform.SkipReconcileTag]("'%s' machine status snapshot not found: %w", machineConfig.Metadata().ID(), err)
		}

		return nil, fmt.Errorf("failed to get machine status snapshot '%s': %w", machineConfig.Metadata().ID(), err)
	}

	machineStatus, err := safe.ReaderGet[*omni.MachineStatus](ctx, r, omni.NewMachineStatus(resources.DefaultNamespace, machineConfig.Metadata().ID()).Metadata())
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, xerrors.NewTaggedf[qtransform.SkipReconcileTag]("'%s' machine status not found: %w", machineConfig.Metadata().ID(), err)
		}

		return nil, fmt.Errorf("failed to get machine status '%s': %w", machineConfig.Metadata().ID(), err)
	}

	if err = checkMachineStatus(ctx, r, machineStatus); err != nil {
		return nil, err
	}

	genOptions, err := safe.ReaderGet[*omni.MachineConfigGenOptions](ctx, r, omni.NewMachineConfigGenOptions(resources.DefaultNamespace, machineConfig.Metadata().ID()).Metadata())
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, xerrors.NewTaggedf[qtransform.SkipReconcileTag]("'%s' machine config gen options not found: %w", machineConfig.Metadata().ID(), err)
		}

		return nil, fmt.Errorf("failed to get install image '%s': %w", machineConfig.Metadata().ID(), err)
	}

	installImage := genOptions.TypedSpec().Value.InstallImage
	if installImage == nil {
		return nil, xerrors.NewTaggedf[qtransform.SkipReconcileTag]("%q install image not found", machineConfig.Metadata().ID())
	}

	schematicMismatch := machineConfigStatus.TypedSpec().Value.SchematicId != installImage.SchematicId

	compareFullSchematicID, err := kernelargs.UpdateSupported(machineStatus, func() (*omni.ClusterMachineConfig, error) {
		return machineConfig, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to determine if kernel args update is supported for machine %q: %w", machineConfig.Metadata().ID(), err)
	}

	if compareFullSchematicID {
		schematicMismatch = schematicMismatch || machineStatus.TypedSpec().Value.Schematic.FullId != installImage.SchematicId
	} else {
		schematicMismatch = schematicMismatch || machineStatus.TypedSpec().Value.Schematic.Id != installImage.SchematicId
	}

	talosVersionMismatch := strings.TrimLeft(machineStatus.TypedSpec().Value.TalosVersion, "v") != machineConfigStatus.TypedSpec().Value.TalosVersion ||
		machineConfigStatus.TypedSpec().Value.TalosVersion != installImage.TalosVersion

	return &ReconciliationContext{
		machineStatus:          machineStatus,
		machineStatusSnapshot:  statusSnapshot,
		machineConfig:          machineConfig,
		machineConfigStatus:    machineConfigStatus,
		shouldUpgrade:          schematicMismatch || talosVersionMismatch,
		compareFullSchematicID: compareFullSchematicID,
		installImage:           installImage,
	}, nil
}
