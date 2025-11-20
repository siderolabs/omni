// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"errors"
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
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/extensions"
	"github.com/siderolabs/omni/internal/backend/kernelargs"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/set"
)

// SchematicConfigurationControllerName is the name of the SchematicConfiguration controller.
const SchematicConfigurationControllerName = "SchematicConfigurationController"

// SchematicConfigurationController combines MachineExtensions resource, MachineStatus overlay into SchematicConfiguration for each existing MachineStatus.
// Ensures schematic exists in the image factory.
type SchematicConfigurationController = qtransform.QController[*omni.MachineStatus, *omni.SchematicConfiguration]

// NewSchematicConfigurationController initializes SchematicConfigurationController.
func NewSchematicConfigurationController(imageFactoryClient ImageFactoryClient) *SchematicConfigurationController {
	helper := &schematicConfigurationHelper{
		imageFactoryClient: imageFactoryClient,
	}

	return qtransform.NewQController(
		qtransform.Settings[*omni.MachineStatus, *omni.SchematicConfiguration]{
			Name: SchematicConfigurationControllerName,
			MapMetadataFunc: func(machineStatus *omni.MachineStatus) *omni.SchematicConfiguration {
				return omni.NewSchematicConfiguration(resources.DefaultNamespace, machineStatus.Metadata().ID())
			},
			UnmapMetadataFunc: func(schematicConfiguration *omni.SchematicConfiguration) *omni.MachineStatus {
				return omni.NewMachineStatus(resources.DefaultNamespace, schematicConfiguration.Metadata().ID())
			},
			TransformExtraOutputFunc: func(ctx context.Context, r controller.ReaderWriter, logger *zap.Logger,
				machineStatus *omni.MachineStatus, schematicConfiguration *omni.SchematicConfiguration,
			) error {
				status, err := helper.reconcile(ctx, r, machineStatus, schematicConfiguration, logger)
				if err != nil {
					return err
				}

				return safe.WriterModify(ctx, r, status, func(r *omni.MachineExtensionsStatus) error {
					helpers.CopyAllLabels(status, r)

					r.TypedSpec().Value = status.TypedSpec().Value

					return nil
				})
			},
			FinalizerRemovalExtraOutputFunc: func(ctx context.Context, r controller.ReaderWriter, _ *zap.Logger, machineStatus *omni.MachineStatus) error {
				statusMD := omni.NewMachineExtensionsStatus(resources.DefaultNamespace, machineStatus.Metadata().ID()).Metadata()

				ready, err := helpers.TeardownAndDestroy(ctx, r, statusMD)
				if err != nil {
					return err
				}

				if !ready {
					return nil
				}

				err = r.RemoveFinalizer(ctx, omni.NewMachineExtensions(resources.DefaultNamespace, machineStatus.Metadata().ID()).Metadata(), SchematicConfigurationControllerName)
				if err != nil && !state.IsNotFoundError(err) {
					return err
				}

				return nil
			},
		},
		qtransform.WithExtraMappedInput[*omni.ClusterMachine](
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
					msPtr := omni.NewMachineStatus(resources.DefaultNamespace, clusterMachine.Metadata().ID())
					ids = append(ids, msPtr.Metadata())
				}

				return ids, nil
			}),
		qtransform.WithExtraMappedInput[*omni.Schematic](
			qtransform.MapperNone(),
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
}

type schematicConfigurationHelper struct {
	imageFactoryClient ImageFactoryClient
}

// Reconcile implements controller.QController interface.
//
//nolint:gocognit,gocyclo,cyclop
func (helper *schematicConfigurationHelper) reconcile(ctx context.Context, r controller.ReaderWriter, ms *omni.MachineStatus,
	schematicConfiguration *omni.SchematicConfiguration, logger *zap.Logger,
) (*omni.MachineExtensionsStatus, error) {
	if ms.TypedSpec().Value.Schematic == nil {
		return nil, xerrors.NewTaggedf[qtransform.SkipReconcileTag]("schematic information is not yet available")
	}

	securityState := ms.TypedSpec().Value.SecurityState
	if securityState == nil {
		return nil, xerrors.NewTaggedf[qtransform.SkipReconcileTag]("security state is not yet available")
	}

	if ms.TypedSpec().Value.TalosVersion == "" {
		return nil, xerrors.NewTaggedf[qtransform.SkipReconcileTag]("machine Talos version is not yet available")
	}

	machineExtensions, err := safe.ReaderGetByID[*omni.MachineExtensions](ctx, r, ms.Metadata().ID())
	if err != nil && !state.IsNotFoundError(err) {
		return nil, err
	}

	if machineExtensions != nil {
		if err = updateFinalizers(ctx, r, machineExtensions); err != nil {
			return nil, err
		}
	}

	cm, err := safe.ReaderGetByID[*omni.ClusterMachine](ctx, r, ms.Metadata().ID())
	if err != nil && !state.IsNotFoundError(err) {
		return nil, err
	}

	var (
		cluster                 *omni.Cluster
		machineExtensionsStatus = omni.NewMachineExtensionsStatus(resources.DefaultNamespace, ms.Metadata().ID())
		talosVersion            = strings.TrimLeft(ms.TypedSpec().Value.TalosVersion, "v")
	)

	if cm != nil && cm.Metadata().Phase() == resource.PhaseRunning { // machine is allocated to a cluster
		clusterName, ok := cm.Metadata().Labels().Get(omni.LabelCluster)
		if !ok {
			return nil, errors.New("failed to determine cluster")
		}

		if cluster, err = safe.ReaderGetByID[*omni.Cluster](ctx, r, clusterName); err != nil {
			return nil, err
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

	machineExtensionsStatus.TypedSpec().Value.TalosVersion = talosVersion

	overlay := getOverlay(ms.TypedSpec().Value.Schematic)

	kernelArgs, kernelArgsUnchanged, err := determineKernelArgs(ctx, ms, r)
	if err != nil {
		return nil, err
	}

	customization, err := newMachineCustomization(ms, cluster, machineExtensions)
	if err != nil {
		return nil, err
	}

	schematicConfiguration.TypedSpec().Value.TalosVersion = talosVersion

	config := schematic.Schematic{
		Customization: schematic.Customization{
			SystemExtensions: schematic.SystemExtensions{
				OfficialExtensions: customization.extensionsList,
			},
			ExtraKernelArgs: kernelArgs,
			Meta:            customization.meta,
		},
		Overlay: overlay,
	}

	id, err := config.ID()
	if err != nil {
		return nil, err
	}

	schematicUnchanged := customization.unchanged && kernelArgsUnchanged
	idMismatch := id != ms.TypedSpec().Value.Schematic.FullId

	// We do a safety check here: if we did not customize anything but the schematic ID appears to have changed,
	// we preserve the existing schematic ID to avoid unnecessary machine reboot.
	// This is an indicator of a bug, we should find out the cause of the discrepancy and fix it.
	if schematicUnchanged && idMismatch {
		logger.Warn("schematic ID mismatch detected despite no changes in schematic configuration, preserving existing schematic ID to avoid unnecessary machine reboot",
			zap.String("machine_schematic_id", ms.TypedSpec().Value.Schematic.FullId),
			zap.String("computed_schematic_id", id),
		)

		id = ms.TypedSpec().Value.Schematic.FullId
		kernelArgs = ms.TypedSpec().Value.Schematic.KernelArgs
	}

	if _, err = safe.ReaderGetByID[*omni.Schematic](ctx, r, id); err != nil {
		if !state.IsNotFoundError(err) {
			return nil, err
		}

		if _, err = helper.imageFactoryClient.EnsureSchematic(ctx, config); err != nil {
			return nil, err
		}
	}

	schematicConfiguration.TypedSpec().Value.SchematicId = id
	schematicConfiguration.TypedSpec().Value.KernelArgs = kernelArgs
	machineExtensionsStatus.TypedSpec().Value.Extensions = computeMachineExtensionsStatus(ms, customization.extensionsList, customization.detectedExtensions)

	return machineExtensionsStatus, nil
}

func getOverlay(schematicConfig *specs.MachineStatusSpec_Schematic) schematic.Overlay {
	if schematicConfig.Overlay == nil {
		return schematic.Overlay{}
	}

	overlay := schematicConfig.Overlay

	return schematic.Overlay{
		Name:  overlay.Name,
		Image: overlay.Image,
	}
}

func updateFinalizers(ctx context.Context, r controller.ReaderWriter, extensions *omni.MachineExtensions) error {
	if extensions.Metadata().Phase() == resource.PhaseTearingDown {
		return r.RemoveFinalizer(ctx, extensions.Metadata(), SchematicConfigurationControllerName)
	}

	if extensions.Metadata().Finalizers().Has(SchematicConfigurationControllerName) {
		return nil
	}

	return r.AddFinalizer(ctx, extensions.Metadata(), SchematicConfigurationControllerName)
}

// newMachineCustomization creates a new machineCustomization based on the given machine status, cluster, and extensions.
//
// MachineStatus is required. All other parameters are optional.
func newMachineCustomization(ms *omni.MachineStatus, cluster *omni.Cluster, exts *omni.MachineExtensions) (machineCustomization, error) {
	mc := machineCustomization{
		machineStatus: ms,
		cluster:       cluster,
	}

	// Leave meta values as-is to prevent any unwanted upgrades.
	// They are no-op for non-UKI machines, but we still preserve them to not cause a schematic ID change, which would upgrade (reboot) the machine.
	mc.meta = xslices.Map(ms.TypedSpec().Value.Schematic.MetaValues,
		func(v *specs.MetaValue) schematic.MetaValue {
			return schematic.MetaValue{
				Key:   uint8(v.GetKey()),
				Value: v.GetValue(),
			}
		})

	if cluster == nil {
		// The machine is not allocated to a cluster, we simply preserve the existing extensions of the machine.
		mc.extensionsList = ms.TypedSpec().Value.Schematic.Extensions

		return mc, nil
	}

	extensionsExplicitlyDefined := exts != nil && exts.Metadata().Phase() == resource.PhaseRunning
	if extensionsExplicitlyDefined {
		mc.machineExtensions = exts
	}

	detected, err := getDetectedExtensions(cluster, ms)
	if err != nil {
		return machineCustomization{}, err
	}

	mc.detectedExtensions = detected

	mc.extensionsList = append(mc.extensionsList, detected...)

	clusterVersion, err := semver.ParseTolerant(cluster.TypedSpec().Value.TalosVersion)
	if err != nil {
		return machineCustomization{}, err
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

	mc.unchanged = slices.Equal(ms.TypedSpec().Value.Schematic.Extensions, mc.extensionsList)

	return mc, nil
}

//nolint:recvcheck
type machineCustomization struct {
	machineStatus      *omni.MachineStatus
	machineExtensions  *omni.MachineExtensions
	cluster            *omni.Cluster
	extensionsList     []string
	detectedExtensions []string
	meta               []schematic.MetaValue
	unchanged          bool
}

func determineKernelArgs(ctx context.Context, machineStatus *omni.MachineStatus, r controller.Reader) (args []string, unchanged bool, err error) {
	if _, kernelArgsInitialized := machineStatus.Metadata().Annotations().Get(omni.KernelArgsInitialized); !kernelArgsInitialized {
		return nil, false, xerrors.NewTaggedf[qtransform.SkipReconcileTag]("kernel args are not yet initialized")
	}

	updateSupported, err := kernelargs.UpdateSupported(machineStatus)
	if err != nil {
		return nil, false, err
	}

	if !updateSupported {
		if machineStatus.TypedSpec().Value.Schematic == nil {
			return nil, false, xerrors.NewTaggedf[qtransform.SkipReconcileTag]("machine schematic is not yet initialized")
		}

		// keep them unchanged - take the current args as-is
		return machineStatus.TypedSpec().Value.Schematic.KernelArgs, true, nil
	}

	kernelArgs, err := kernelargs.GetUncached(ctx, r, machineStatus.Metadata().ID())
	if err != nil && !state.IsNotFoundError(err) {
		return nil, false, err
	}

	args, ok, err := kernelargs.Calculate(machineStatus, kernelArgs)
	if err != nil {
		return nil, false, err
	}

	if !ok {
		return nil, false, xerrors.NewTaggedf[qtransform.SkipReconcileTag]("extra kernel args for machine %q are not yet initialized", machineStatus.Metadata().ID())
	}

	return args, false, nil
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

func computeMachineExtensionsStatus(ms *omni.MachineStatus, extensionsList, detectedExtensions []string) []*specs.MachineExtensionsStatusSpec_Item {
	var installedExtensions, requestedExtensions set.Set[string]

	installedExtensions = xslices.ToSet(ms.TypedSpec().Value.Schematic.Extensions)
	requestedExtensions = xslices.ToSet(extensionsList)

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

		switch {
		// detected extensions should always be pre installed
		case slices.Contains(detectedExtensions, extension):
			phase = specs.MachineExtensionsStatusSpec_Item_Installed

			immutable = true
		case installedExtensions.Contains(extension) && !requestedExtensions.Contains(extension):
			phase = specs.MachineExtensionsStatusSpec_Item_Removing
		case !installedExtensions.Contains(extension) && requestedExtensions.Contains(extension):
			phase = specs.MachineExtensionsStatusSpec_Item_Installing
		}

		statusExtensions = append(statusExtensions,
			&specs.MachineExtensionsStatusSpec_Item{
				Name:      extension,
				Immutable: immutable,
				Phase:     phase,
			},
		)
	}

	return statusExtensions
}
