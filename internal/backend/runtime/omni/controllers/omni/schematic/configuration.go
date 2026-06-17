// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package schematic

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xerrors"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/image-factory/pkg/schematic"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/extensions"
	"github.com/siderolabs/omni/internal/backend/imagefactory"
	"github.com/siderolabs/omni/internal/backend/kernelargs"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/set"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/uncached"
)

// ConfigurationControllerName is the name of the SchematicConfiguration controller.
const ConfigurationControllerName = "SchematicConfigurationController"

// Ensurer can ensure a schematic exists in the image factory, i.e., an image factory client.
type Ensurer interface {
	EnsureSchematic(ctx context.Context, inputSchematic schematic.Schematic) (string, *schematic.Schematic, error)
}

// ConfigurationController combines MachineExtensions resource, MachineStatus overlay into SchematicConfiguration for each existing MachineStatus.
// Ensures schematic exists in the image factory.
type ConfigurationController struct {
	*qtransform.QController[*omni.MachineStatus, *omni.SchematicConfiguration]
	imageFactoryClient Ensurer
}

// NewConfigurationController initializes SchematicConfigurationController.
func NewConfigurationController(imageFactoryClient Ensurer) *ConfigurationController {
	ctrl := &ConfigurationController{
		imageFactoryClient: imageFactoryClient,
	}

	ctrl.QController = qtransform.NewQController(
		qtransform.Settings[*omni.MachineStatus, *omni.SchematicConfiguration]{
			Name: ConfigurationControllerName,
			MapMetadataFunc: func(machineStatus *omni.MachineStatus) *omni.SchematicConfiguration {
				return omni.NewSchematicConfiguration(machineStatus.Metadata().ID())
			},
			UnmapMetadataFunc: func(schematicConfiguration *omni.SchematicConfiguration) *omni.MachineStatus {
				return omni.NewMachineStatus(schematicConfiguration.Metadata().ID())
			},
			TransformExtraOutputFunc:        ctrl.transform,
			FinalizerRemovalExtraOutputFunc: ctrl.finalizerRemoval,
		},
		qtransform.WithExtraMappedInput[*omni.ClusterMachine](
			qtransform.MapperSameID[*omni.MachineStatus](),
		),
		qtransform.WithExtraMappedInput[*omni.ClusterMachineConfig](
			qtransform.MapperSameID[*omni.MachineStatus](),
		),
		qtransform.WithExtraMappedInput[*omni.Cluster](
			func(ctx context.Context, logger *zap.Logger, runtime controller.QRuntime, metadata controller.ReducedResourceMetadata) ([]resource.Pointer, error) {
				clusterID := metadata.ID()

				cmList, err := safe.ReaderListAll[*omni.ClusterMachine](ctx, runtime, state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, clusterID)))
				if err != nil {
					return nil, err
				}

				ids := make([]resource.Pointer, 0, cmList.Len())

				for clusterMachine := range cmList.All() {
					msPtr := omni.NewMachineStatus(clusterMachine.Metadata().ID())
					ids = append(ids, msPtr.Metadata())
				}

				return ids, nil
			},
		),
		qtransform.WithExtraMappedInput[*omni.MachineExtensions](
			qtransform.MapperSameID[*omni.MachineStatus](),
		),
		qtransform.WithExtraMappedInput[*omni.KernelArgs](
			qtransform.MapperSameID[*omni.MachineStatus](),
		),
		qtransform.WithExtraOutputs(
			controller.Output{
				Type: omni.MachineExtensionsStatusType,
				Kind: controller.OutputExclusive,
			},
		),
	)

	return ctrl
}

func (ctrl *ConfigurationController) saveMachineExtensionStatus(ctx context.Context, r controller.ReaderWriter, status *omni.MachineExtensionsStatus) error {
	return safe.WriterModify(ctx, r, omni.NewMachineExtensionsStatus(status.Metadata().ID()), func(res *omni.MachineExtensionsStatus) error {
		helpers.SyncAllLabels(status, res)

		res.TypedSpec().Value = status.TypedSpec().Value

		return nil
	})
}

func (ctrl *ConfigurationController) transform(ctx context.Context, r controller.ReaderWriter,
	_ *zap.Logger, ms *omni.MachineStatus, schematicConfiguration *omni.SchematicConfiguration,
) error {
	machineExtensions, err := safe.ReaderGetByID[*omni.MachineExtensions](ctx, r, ms.Metadata().ID())
	if err != nil && !state.IsNotFoundError(err) {
		return err
	}

	if machineExtensions != nil {
		if err = ctrl.updateFinalizers(ctx, r, machineExtensions); err != nil {
			return err
		}
	}

	if !ms.TypedSpec().Value.SchematicReady() {
		return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("schematic is not yet ready")
	}

	securityState := ms.TypedSpec().Value.SecurityState
	if securityState == nil {
		return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("security state is not yet available")
	}

	if ms.TypedSpec().Value.TalosVersion == "" {
		return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("machine Talos version is not yet available")
	}

	cm, err := safe.ReaderGetByID[*omni.ClusterMachine](ctx, r, ms.Metadata().ID())
	if err != nil && !state.IsNotFoundError(err) {
		return err
	}

	var (
		cluster                 *omni.Cluster
		machineExtensionsStatus = omni.NewMachineExtensionsStatus(ms.Metadata().ID())
		talosVersion            = strings.TrimLeft(ms.TypedSpec().Value.TalosVersion, "v")
	)

	if cm != nil && cm.Metadata().Phase() == resource.PhaseRunning { // machine is allocated to a cluster
		clusterName, ok := cm.Metadata().Labels().Get(omni.LabelCluster)
		if !ok {
			return errors.New("failed to determine cluster")
		}

		if cluster, err = safe.ReaderGetByID[*omni.Cluster](ctx, r, clusterName); err != nil {
			return err
		}

		talosVersion = cluster.TypedSpec().Value.TalosVersion

		schematicConfiguration.Metadata().Labels().Set(omni.LabelCluster, clusterName)
		helpers.CopyLabels(cm, machineExtensionsStatus, omni.LabelCluster, omni.LabelWorkerRole, omni.LabelControlPlaneRole, omni.LabelMachineSet)
	} else {
		schematicConfiguration.Metadata().Labels().Delete(omni.LabelCluster)
		machineExtensionsStatus.Metadata().Labels().Delete(omni.LabelCluster)
		machineExtensionsStatus.Metadata().Labels().Delete(omni.LabelWorkerRole)
		machineExtensionsStatus.Metadata().Labels().Delete(omni.LabelControlPlaneRole)
		machineExtensionsStatus.Metadata().Labels().Delete(omni.LabelMachineSet)
	}

	machineExtensionsStatus.TypedSpec().Value.TalosVersion = "v" + talosVersion

	// Invalid machines bypassed the image factory entirely (extensions baked into a custom Talos build).
	// They have no usable schematic info, but downstream controllers still need a SchematicConfiguration
	// resource to exist so the install image / config generation pipeline runs. Emit a minimal one and
	// let the downstream's existing Invalid handling (installimage.Build falls back to talosRegistry:version,
	// ReconciliationContext skips schematic mismatch) take over.
	if ms.TypedSpec().Value.Schematic.Invalid {
		schematicConfiguration.TypedSpec().Value.TalosVersion = talosVersion
		schematicConfiguration.TypedSpec().Value.SchematicId = ""

		return ctrl.saveMachineExtensionStatus(ctx, r, machineExtensionsStatus)
	}

	rawSchematic := ms.TypedSpec().Value.Schematic.GetRaw()
	if rawSchematic == "" {
		return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("machine schematic raw YAML is not yet available")
	}

	kernelArgs, err := determineKernelArgs(ctx, ms, r)
	if err != nil {
		return err
	}

	customization, err := newMachineCustomization(ctx, r, ms, cluster)
	if err != nil {
		return err
	}

	schematicConfiguration.TypedSpec().Value.TalosVersion = talosVersion

	// Patch only the fields Omni manages (extensions list, kernel args). Everything else
	// the user/factory put into the schematic (owner, secureboot, bootloader, meta,
	// overlay incl. options, future fields) is preserved by going through the raw YAML
	// the machine reports.
	patched, err := imagefactory.PatchSchematic(rawSchematic, customization.extensionsList, kernelArgs)
	if err != nil {
		return fmt.Errorf("failed to patch schematic: %w", err)
	}

	// TODO(preserve-schematic): when the patched schematic is content-equal to the
	// source raw (i.e. no Omni-driven customization changed anything), short-circuit
	// the factory call and publish ms.Schematic.FullId directly.
	id, _, err := ctrl.imageFactoryClient.EnsureSchematic(ctx, patched)
	if err != nil {
		return err
	}

	schematicConfiguration.TypedSpec().Value.SchematicId = id
	machineExtensionsStatus.TypedSpec().Value.Extensions = computeMachineExtensionsStatus(ms, &customization)

	return ctrl.saveMachineExtensionStatus(ctx, r, machineExtensionsStatus)
}

func (ctrl *ConfigurationController) finalizerRemoval(ctx context.Context, r controller.ReaderWriter, _ *zap.Logger, machineStatus *omni.MachineStatus) error {
	statusMD := omni.NewMachineExtensionsStatus(machineStatus.Metadata().ID()).Metadata()

	ready, err := helpers.TeardownAndDestroy(ctx, r, statusMD)
	if err != nil {
		return err
	}

	if !ready {
		return nil
	}

	err = r.RemoveFinalizer(ctx, omni.NewMachineExtensions(machineStatus.Metadata().ID()).Metadata(), ConfigurationControllerName)
	if err != nil && !state.IsNotFoundError(err) {
		return err
	}

	return nil
}

func (ctrl *ConfigurationController) updateFinalizers(ctx context.Context, r controller.ReaderWriter, extensions *omni.MachineExtensions) error {
	if extensions.Metadata().Phase() == resource.PhaseTearingDown {
		return r.RemoveFinalizer(ctx, extensions.Metadata(), ConfigurationControllerName)
	}

	if extensions.Metadata().Finalizers().Has(ctrl.Name()) {
		return nil
	}

	return r.AddFinalizer(ctx, extensions.Metadata(), ctrl.Name())
}

// newMachineCustomization creates a new machineCustomization based on the given machine status and cluster.
//
// MachineStatus is required. Cluster is optional — when nil, the machine's existing extensions are preserved.
// When cluster is non-nil, MachineExtensions is read bypassing the resource cache to avoid stale reads
// that could produce a wrong schematic ID, triggering an unwanted Talos upgrade on the node.
func newMachineCustomization(ctx context.Context, r controller.Reader, ms *omni.MachineStatus, cluster *omni.Cluster) (machineCustomization, error) {
	mc := machineCustomization{
		machineStatus: ms,
		cluster:       cluster,
	}

	if cluster == nil {
		// The machine is not allocated to a cluster, we simply preserve the existing extensions of the machine.
		mc.extensionsList = ms.TypedSpec().Value.Schematic.Extensions

		return mc, nil
	}

	machineExtensions, err := safe.ReaderGetByID[*omni.MachineExtensions](ctx, uncached.Reader(r), ms.Metadata().ID())
	if err != nil && !state.IsNotFoundError(err) {
		return mc, err
	}

	extensionsExplicitlyDefined := machineExtensions != nil && machineExtensions.Metadata().Phase() == resource.PhaseRunning
	if extensionsExplicitlyDefined {
		mc.machineExtensions = machineExtensions
	}

	detected, err := getDetectedExtensions(cluster, ms)
	if err != nil {
		return mc, err
	}

	mc.detectedExtensions = detected

	mc.extensionsList = append(mc.extensionsList, detected...)

	clusterVersion, err := semver.ParseTolerant(cluster.TypedSpec().Value.TalosVersion)
	if err != nil {
		return mc, err
	}

	if mc.machineExtensions != nil {
		for _, e := range mc.machineExtensions.TypedSpec().Value.Extensions {
			if slices.Contains(mc.extensionsList, e) {
				continue
			}

			mc.extensionsList = append(mc.extensionsList, e)
		}
	}

	mc.extensionsList = extensions.MapNamesByVersion(mc.extensionsList, clusterVersion)

	slices.Sort(mc.extensionsList)

	if !extensionsExplicitlyDefined && len(mc.extensionsList) == 0 { // extensions are not explicitly set, we should revert them to their initial state
		initialState := ms.TypedSpec().Value.Schematic.InitialState
		if initialState == nil {
			return mc, xerrors.NewTaggedf[qtransform.SkipReconcileTag]("machine initial schematic state is not yet set")
		}

		mc.extensionsList = initialState.Extensions
	}

	return mc, nil
}

type machineCustomization struct {
	machineStatus      *omni.MachineStatus
	machineExtensions  *omni.MachineExtensions
	cluster            *omni.Cluster
	extensionsList     []string
	detectedExtensions []string
}

func determineKernelArgs(ctx context.Context, machineStatus *omni.MachineStatus, r controller.Reader) ([]string, error) {
	if _, kernelArgsInitialized := machineStatus.Metadata().Annotations().Get(omni.KernelArgsInitialized); !kernelArgsInitialized {
		return nil, xerrors.NewTaggedf[qtransform.SkipReconcileTag]("kernel args are not yet initialized")
	}

	updateSupported, err := kernelargs.UpdateSupported(machineStatus, func() (*omni.ClusterMachineConfig, error) {
		return safe.ReaderGetByID[*omni.ClusterMachineConfig](ctx, r, machineStatus.Metadata().ID())
	})
	if err != nil {
		return nil, err
	}

	if !updateSupported {
		if !machineStatus.TypedSpec().Value.SchematicReady() {
			return nil, xerrors.NewTaggedf[qtransform.SkipReconcileTag]("machine schematic is not yet ready")
		}

		// keep them unchanged - take the current args as-is
		return machineStatus.TypedSpec().Value.Schematic.KernelArgs, nil
	}

	// Read the KernelArgs resource with the given ID, bypassing the resource cache.
	// We need this to avoid stale reads of the KernelArgs resource: there can be cases where the omni.KernelArgsInitialized annotation is present in the MachineStatus,
	// but the KernelArgs resource is not yet visible due to the resource cache, which can cause unwanted Talos upgrades through a schematic id update.
	kernelArgs, err := safe.ReaderGetByID[*omni.KernelArgs](ctx, uncached.Reader(r), machineStatus.Metadata().ID())

	if err != nil && !state.IsNotFoundError(err) {
		return nil, err
	}

	args, ok, err := kernelargs.Calculate(machineStatus, kernelArgs)
	if err != nil {
		return nil, err
	}

	if !ok {
		return nil, xerrors.NewTaggedf[qtransform.SkipReconcileTag]("extra kernel args for machine %q are not yet initialized", machineStatus.Metadata().ID())
	}

	return args, nil
}

func (mc *machineCustomization) isDetected(value string) bool {
	return slices.Contains(mc.detectedExtensions, value)
}

func getDetectedExtensions(cluster *omni.Cluster, machineStatus *omni.MachineStatus) ([]string, error) {
	if machineStatus.TypedSpec().Value.InitialTalosVersion == "" {
		return nil, xerrors.NewTaggedf[qtransform.SkipReconcileTag]("machine initial version is not set")
	}

	initialVersion, err := semver.ParseTolerant(machineStatus.TypedSpec().Value.InitialTalosVersion)
	if err != nil {
		return nil, err
	}

	currentVersion, err := semver.ParseTolerant(cluster.TypedSpec().Value.TalosVersion)
	if err != nil {
		return nil, err
	}

	currentVersion.Pre = nil

	return detectExtensions(initialVersion, currentVersion)
}

func detectExtensions(initialVersion, currentVersion semver.Version) ([]string, error) {
	// on 1.5.x these extensions were part of the kernel
	// automatically install them when upgrading to 1.6.x+
	if initialVersion.Major == 1 && initialVersion.Minor == 5 && currentVersion.GTE(semver.MustParse("1.6.0")) {
		// install firmware
		return []string{
			extensions.OfficialPrefix + "bnx2-bnx2x",
			extensions.OfficialPrefix + "intel-ice-firmware",
		}, nil
	}

	return nil, nil
}

func computeMachineExtensionsStatus(ms *omni.MachineStatus, customization *machineCustomization) []*specs.MachineExtensionsStatusSpec_Item {
	var (
		installedExtensions set.Set[string]
		requestedExtensions set.Set[string]
	)

	if ms.TypedSpec().Value.SchematicReady() {
		installedExtensions = xslices.ToSet(ms.TypedSpec().Value.Schematic.Extensions)
	}

	if customization != nil {
		requestedExtensions = xslices.ToSet(customization.extensionsList)
	}

	allExtensions := set.Values(set.Union(
		installedExtensions,
		requestedExtensions,
	))

	statusExtensions := make([]*specs.MachineExtensionsStatusSpec_Item, 0, len(allExtensions))

	for _, extension := range allExtensions {
		var (
			immutable bool
			phase     specs.MachineExtensionsStatusSpec_Item_Phase
		)

		if customization != nil {
			switch {
			// detected extensions should always be pre installed
			case customization.isDetected(extension):
				phase = specs.MachineExtensionsStatusSpec_Item_Installed

				immutable = true
			case installedExtensions.Contains(extension) && !requestedExtensions.Contains(extension):
				phase = specs.MachineExtensionsStatusSpec_Item_Removing
			case !installedExtensions.Contains(extension) && requestedExtensions.Contains(extension):
				phase = specs.MachineExtensionsStatusSpec_Item_Installing
			}
		}

		statusExtensions = append(
			statusExtensions,
			&specs.MachineExtensionsStatusSpec_Item{
				Name:      extension,
				Immutable: immutable,
				Phase:     phase,
			},
		)
	}

	return statusExtensions
}
