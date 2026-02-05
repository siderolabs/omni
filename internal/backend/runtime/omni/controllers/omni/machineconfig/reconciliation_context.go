// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package machineconfig

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/siderolabs/crypto/x509"
	"github.com/siderolabs/gen/xerrors"
	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/kernelargs"
)

// ReconciliationContext describes all related data for one reconciliation call of the machine config status controller.
type ReconciliationContext struct {
	machineStatus         *omni.MachineStatus
	machineConfig         *omni.ClusterMachineConfig
	machineConfigStatus   *omni.ClusterMachineConfigStatus
	machineStatusSnapshot *omni.MachineStatusSnapshot
	clusterMachine        *omni.ClusterMachine
	installImage          *specs.MachineConfigGenOptionsSpec_InstallImage

	lastConfigError       string
	redactedMachineConfig []byte

	shouldUpgrade          bool
	compareFullSchematicID bool
	configUpdatesAllowed   bool
	locked                 bool
}

func checkClusterReady(ctx context.Context, r controller.Reader, machineConfig *omni.ClusterMachineConfig) (bool, error) {
	clusterName, ok := machineConfig.Metadata().Labels().Get(omni.LabelCluster)
	if !ok {
		return false, nil
	}

	cluster, err := safe.ReaderGetByID[*omni.Cluster](ctx, r, clusterName)
	if err != nil {
		if state.IsNotFoundError(err) {
			return false, xerrors.NewTaggedf[qtransform.SkipReconcileTag]("cluster doesn't exist")
		}

		return false, err
	}

	if _, locked := cluster.Metadata().Annotations().Get(omni.ClusterLocked); locked && cluster.Metadata().Phase() == resource.PhaseRunning {
		return true, nil
	}

	return false, nil
}

func checkMachineStatus(ctx context.Context, r controller.Reader, machineStatus *omni.MachineStatus) error {
	if !machineStatus.TypedSpec().Value.Connected {
		return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("machine %q is not connected", machineStatus.Metadata().ID())
	}

	if !machineStatus.TypedSpec().Value.SchematicReady() {
		return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("machine %q schematic is not ready", machineStatus.Metadata().ID())
	}

	// if the machine is managed by a static infra provider, we need to ensure that the infra machine is ready to use
	if _, isManagedByStaticInfraProvider := machineStatus.Metadata().Labels().Get(omni.LabelIsManagedByStaticInfraProvider); isManagedByStaticInfraProvider {
		var infraMachineStatus *infra.MachineStatus

		infraMachineStatus, err := safe.ReaderGetByID[*infra.MachineStatus](ctx, r, machineStatus.Metadata().ID())
		if err != nil {
			if state.IsNotFoundError(err) {
				return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("machine is managed by a static infra provider but the infra machine status is not found: %w", err)
			}

			return fmt.Errorf("failed to get infra machine status %q: %w", machineStatus.Metadata().ID(), err)
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
func (rc *ReconciliationContext) SchematicEqual(extensionsOnlyID, fullID, expectedSchematic string) bool {
	if rc.compareFullSchematicID {
		return fullID == expectedSchematic
	}

	return extensionsOnlyID == expectedSchematic || fullID == expectedSchematic
}

// BuildReconciliationContext is the COSI reader dependent method to build the reconciliation context.
//
//nolint:gocognit,gocyclo,cyclop
func BuildReconciliationContext(ctx context.Context, r controller.Reader,
	machineConfig *omni.ClusterMachineConfig, machineConfigStatus *omni.ClusterMachineConfigStatus,
) (*ReconciliationContext, error) {
	desiredConfig, err := machineConfig.TypedSpec().Value.GetUncompressedData()
	if err != nil {
		return nil, fmt.Errorf("failed to read desired config: %w", err)
	}

	defer desiredConfig.Free()

	machineSetNode, err := safe.ReaderGetByID[*omni.MachineSetNode](ctx, r, machineConfig.Metadata().ID())
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, xerrors.NewTaggedf[qtransform.SkipReconcileTag]("%q machine set node not found: %w", machineConfig.Metadata().ID(), err)
		}

		return nil, err
	}

	rc := &ReconciliationContext{
		machineConfig:       machineConfig,
		machineConfigStatus: machineConfigStatus,
	}

	_, locked := machineSetNode.Metadata().Annotations().Get(omni.MachineLocked)
	// we also check config SHA not being empty here as we don't want to block the initial config creation
	rc.locked = locked && machineConfigStatus.TypedSpec().Value.ClusterMachineConfigSha256 != ""

	rc.lastConfigError = machineConfig.TypedSpec().Value.GenerationError

	if rc.lastConfigError != "" {
		return rc, nil
	}

	config, err := configloader.NewFromBytes(desiredConfig.Data())
	if err != nil {
		return nil, fmt.Errorf("failed to decode desired config: %w", err)
	}

	rc.redactedMachineConfig, err = config.RedactSecrets(x509.Redacted).EncodeBytes(encoder.WithComments(encoder.CommentsDisabled))
	if err != nil {
		return nil, fmt.Errorf("failed to redact secrets: %w", err)
	}

	if !rc.locked {
		if rc.locked, err = checkClusterReady(ctx, r, machineConfig); err != nil {
			return nil, err
		}
	}

	rc.machineStatusSnapshot, err = safe.ReaderGetByID[*omni.MachineStatusSnapshot](ctx, r,
		machineConfig.Metadata().ID(),
	)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, xerrors.NewTaggedf[qtransform.SkipReconcileTag]("%q machine status snapshot not found: %w", machineConfig.Metadata().ID(), err)
		}

		return nil, fmt.Errorf("failed to get machine status snapshot %q: %w", machineConfig.Metadata().ID(), err)
	}

	rc.machineStatus, err = safe.ReaderGetByID[*omni.MachineStatus](ctx, r, machineConfig.Metadata().ID())
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, xerrors.NewTaggedf[qtransform.SkipReconcileTag]("%q machine status not found: %w", machineConfig.Metadata().ID(), err)
		}

		return nil, fmt.Errorf("failed to get machine status %q: %w", machineConfig.Metadata().ID(), err)
	}

	if err = checkMachineStatus(ctx, r, rc.machineStatus); err != nil {
		return nil, err
	}

	genOptions, err := safe.ReaderGetByID[*omni.MachineConfigGenOptions](ctx, r, machineConfig.Metadata().ID())
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, xerrors.NewTaggedf[qtransform.SkipReconcileTag]("%q machine config gen options not found: %w", machineConfig.Metadata().ID(), err)
		}

		return nil, fmt.Errorf("failed to get install image %q: %w", machineConfig.Metadata().ID(), err)
	}

	rc.installImage = genOptions.TypedSpec().Value.InstallImage
	if rc.installImage == nil {
		return nil, xerrors.NewTaggedf[qtransform.SkipReconcileTag]("%q install image not found", machineConfig.Metadata().ID())
	}

	schematicMismatch := machineConfigStatus.TypedSpec().Value.SchematicId != rc.installImage.SchematicId

	rc.compareFullSchematicID, err = kernelargs.UpdateSupported(rc.machineStatus, func() (*omni.ClusterMachineConfig, error) {
		return machineConfig, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to determine if kernel args update is supported for machine %q: %w", machineConfig.Metadata().ID(), err)
	}

	schematicMismatch = schematicMismatch || !rc.SchematicEqual(rc.machineStatus.TypedSpec().Value.Schematic.Id, rc.machineStatus.TypedSpec().Value.Schematic.FullId, rc.installImage.SchematicId)

	talosVersionMismatch := strings.TrimLeft(rc.machineStatus.TypedSpec().Value.TalosVersion, "v") != machineConfigStatus.TypedSpec().Value.TalosVersion ||
		machineConfigStatus.TypedSpec().Value.TalosVersion != rc.installImage.TalosVersion

	machineSetName, ok := machineConfig.Metadata().Labels().Get(omni.LabelMachineSet)
	if !ok {
		return nil, errors.New("failed to get machine set name from the machine config resource")
	}

	machineSetStatus, err := safe.ReaderGetByID[*omni.MachineSetStatus](ctx, r, machineSetName)
	if err != nil && !state.IsNotFoundError(err) {
		return nil, err
	}

	rc.configUpdatesAllowed = true

	if machineSetStatus != nil {
		rc.configUpdatesAllowed = machineSetStatus.TypedSpec().Value.ConfigUpdatesAllowed
	}

	rc.clusterMachine, err = safe.ReaderGetByID[*omni.ClusterMachine](ctx, r, machineConfig.Metadata().ID())
	if err != nil && !state.IsNotFoundError(err) {
		return nil, err
	}

	rc.shouldUpgrade = schematicMismatch || talosVersionMismatch

	return rc, nil
}

func computeDiff(previousData, newData []byte) (string, error) {
	if len(previousData) != 0 && len(newData) != 0 {
		previousConfig, err := configloader.NewFromBytes(previousData)
		if err != nil {
			return "", err
		}

		newConfig, err := configloader.NewFromBytes(newData)
		if err != nil {
			return "", err
		}

		// exclude the install section from the diff, as this part is handled by the upgrades
		if previousConfig.RawV1Alpha1() != nil {
			if previousConfig, err = previousConfig.PatchV1Alpha1(func(c *v1alpha1.Config) error {
				cfg := newConfig.RawV1Alpha1()
				if cfg == nil {
					return nil
				}

				c.MachineConfig.MachineInstall = newConfig.RawV1Alpha1().MachineConfig.MachineInstall

				return nil
			}); err != nil {
				return "", err
			}
		}

		previousData, err = previousConfig.EncodeBytes(encoder.WithComments(encoder.CommentsDisabled))
		if err != nil {
			return "", err
		}
	}

	oldConfigString := string(previousData)
	newConfigString := string(newData)

	edits := myers.ComputeEdits("", oldConfigString, newConfigString)
	diff := gotextdiff.ToUnified("", "", oldConfigString, edits)
	diffStr := strings.TrimSpace(fmt.Sprint(diff))
	diffStr = strings.Replace(diffStr, "--- \n+++ \n", "", 1) // trim the URIs, as they do not make sense in this context

	if strings.TrimSpace(diffStr) == "" {
		return "", nil
	}

	return diffStr, nil
}
