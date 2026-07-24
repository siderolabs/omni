// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package machineconfig

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/crypto/x509"
	"github.com/siderolabs/gen/xerrors"
	talosconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/cri"
	"github.com/siderolabs/talos/pkg/machinery/config/types/security"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	siderolinkres "github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/uncached"
	"github.com/siderolabs/omni/internal/backend/talos/lifecycle"
)

// ReconciliationContext describes all related data for one reconciliation call of the machine config status controller.
type ReconciliationContext struct {
	machineStatus         *omni.MachineStatus
	machineConfig         *omni.ClusterMachineConfig
	machineConfigStatus   *omni.ClusterMachineConfigStatus
	machineStatusSnapshot *omni.MachineStatusSnapshot
	clusterMachine        *omni.ClusterMachine
	installImage          *specs.MachineConfigGenOptionsSpec_InstallImage

	lastConfigError string
	installDisk     string
	bootID          string
	// highPriorityHash is the hash of the high-priority config documents (image factory registry auth
	// and custom CA / trusted roots) present in the desired config. Empty when there are none.
	highPriorityHash      string
	redactedMachineConfig []byte

	lifecycleOp lifecycle.Op

	configUpdatesAllowed bool
	locked               bool
	// highPriorityPending is true when the desired high-priority config documents differ from the ones
	// last applied to the machine, so they must be applied before any upgrade/install.
	highPriorityPending bool
	// maintenanceConfigApplied is true when the maintenance config controller has applied the config it
	// generated (which carries the high-priority documents) for the machine's current connection.
	maintenanceConfigApplied bool
}

// hasPendingLifecycleOperation returns true when an upgrade/install action needs to run before config can be applied.
func (rc *ReconciliationContext) hasPendingLifecycleOperation() bool {
	return rc.lifecycleOp != lifecycle.OpNone
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

		// Interim guard: right after a wipe the machine still reports its old system disk, so DecideOp
		// treats it as installed and runs config-apply, whose install races the LifecycleService install
		// and corrupts the disk. Wait until Omni also sees no disk. The provider Installed flag is wipe-aware.
		// TODO: drop once .machine.install is removed from the config for Talos >= 1.13 clusters.
		if !infraMachineStatus.TypedSpec().Value.Installed && omni.GetMachineStatusSystemDisk(machineStatus) != "" {
			return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("machine was wiped but still reports a stale system disk")
		}
	}

	return nil
}

func (rc *ReconciliationContext) ID() string {
	return rc.machineConfig.Metadata().ID()
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

	rc.highPriorityHash, err = computeHighPriorityHash(config.Documents())
	if err != nil {
		return nil, fmt.Errorf("failed to compute high-priority config hash: %w", err)
	}

	rc.highPriorityPending = rc.highPriorityHash != "" &&
		rc.highPriorityHash != machineConfigStatus.TypedSpec().Value.AppliedHighPriorityConfigHash

	if !rc.locked {
		if rc.locked, err = checkClusterReady(ctx, r, machineConfig); err != nil {
			return nil, err
		}
	}

	rc.machineStatusSnapshot, err = safe.ReaderGetByID[*omni.MachineStatusSnapshot](
		ctx, r,
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

	rc.installDisk = genOptions.TypedSpec().Value.InstallDisk

	schematicMismatch := false

	// Invalid schematic means the machine was not provisioned via image factory.
	// Skip schematic comparison — schematic plays no role in upgrade decisions for these machines.
	if !rc.machineStatus.TypedSpec().Value.Schematic.Invalid {
		schematicMismatch = machineConfigStatus.TypedSpec().Value.SchematicId != rc.installImage.SchematicId ||
			rc.machineStatus.TypedSpec().Value.Schematic.FullId != rc.installImage.SchematicId
	}

	talosVersionMismatch := strings.TrimLeft(rc.machineStatus.TypedSpec().Value.TalosVersion, "v") != machineConfigStatus.TypedSpec().Value.TalosVersion ||
		machineConfigStatus.TypedSpec().Value.TalosVersion != rc.installImage.TalosVersion

	machineSetName, ok := machineConfig.Metadata().Labels().Get(omni.LabelMachineSet)
	if !ok {
		return nil, errors.New("failed to get machine set name from the machine config resource")
	}

	// A stale allow value could apply a config while updates are blocked.
	machineSetConfigStatus, err := safe.ReaderGetByID[*omni.MachineSetConfigStatus](ctx, uncached.Reader(r), machineSetName)
	if err != nil && !state.IsNotFoundError(err) {
		return nil, err
	}

	rc.configUpdatesAllowed = true

	if machineSetConfigStatus != nil {
		rc.configUpdatesAllowed = machineSetConfigStatus.TypedSpec().Value.ConfigUpdatesAllowed
	}

	rc.clusterMachine, err = safe.ReaderGetByID[*omni.ClusterMachine](ctx, r, machineConfig.Metadata().ID())
	if err != nil && !state.IsNotFoundError(err) {
		return nil, err
	}

	rc.bootID = rc.machineStatusSnapshot.TypedSpec().Value.BootId

	maintenanceConfigStatus, err := safe.ReaderGetByID[*omni.MaintenanceConfigStatus](ctx, r, machineConfig.Metadata().ID())
	if err != nil && !state.IsNotFoundError(err) {
		return nil, err
	}

	link, err := safe.ReaderGetByID[*siderolinkres.Link](ctx, r, machineConfig.Metadata().ID())
	if err != nil && !state.IsNotFoundError(err) {
		return nil, err
	}

	rc.maintenanceConfigApplied = maintenanceConfigStatus != nil && link != nil &&
		link.TypedSpec().Value.Connected &&
		link.TypedSpec().Value.NodePublicKey != "" &&
		maintenanceConfigStatus.TypedSpec().Value.LastAppliedConfigHash != "" &&
		maintenanceConfigStatus.TypedSpec().Value.PublicKeyAtLastApply != "" &&
		maintenanceConfigStatus.TypedSpec().Value.PublicKeyAtLastApply == link.TypedSpec().Value.NodePublicKey

	rc.lifecycleOp = lifecycle.DecideOp(rc.machineStatus, rc.installImage, schematicMismatch, talosVersionMismatch)
	if rc.lifecycleOp == lifecycle.OpMaintenanceInstall && rc.installDisk == "" {
		return nil, xerrors.NewTaggedf[qtransform.SkipReconcileTag]("%q install disk is not yet selected", machineConfig.Metadata().ID())
	}

	return rc, nil
}

// computeHighPriorityHash returns a stable hash of the high-priority config documents present in the
// desired machine config: image factory registry auth (cri.RegistryAuthConfigV1Alpha1) and custom CA /
// trusted roots (security.TrustedRootsConfigV1Alpha1). These must already be on the machine before any
// upgrade/install can pull the installer image from a private image factory or a registry behind a
// custom CA. It returns an empty string when there are no such documents.
func computeHighPriorityHash(documents []talosconfig.Document) (string, error) {
	var highPriority []talosconfig.Document

	for _, doc := range documents {
		switch doc.(type) {
		case *cri.RegistryAuthConfigV1Alpha1, *security.TrustedRootsConfigV1Alpha1:
			highPriority = append(highPriority, doc)
		}
	}

	if len(highPriority) == 0 {
		return "", nil
	}

	// Sort by kind and name so the hash is independent of the document ordering in the config.
	slices.SortStableFunc(highPriority, func(a, b talosconfig.Document) int {
		return strings.Compare(documentKey(a), documentKey(b))
	})

	hash := sha256.New()

	for _, doc := range highPriority {
		ctr, err := container.New(doc)
		if err != nil {
			return "", fmt.Errorf("failed to wrap high-priority document: %w", err)
		}

		data, err := ctr.EncodeBytes(encoder.WithComments(encoder.CommentsDisabled))
		if err != nil {
			return "", fmt.Errorf("failed to encode high-priority document: %w", err)
		}

		hash.Write(data)
		hash.Write([]byte{0})
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// documentKey returns a stable sort key for a config document based on its API version, kind and
// (optional) name.
func documentKey(doc talosconfig.Document) string {
	key := doc.APIVersion() + "/" + doc.Kind()

	if named, ok := doc.(talosconfig.NamedDocument); ok {
		key += "/" + named.Name()
	}

	return key
}
