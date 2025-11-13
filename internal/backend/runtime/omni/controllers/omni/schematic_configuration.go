// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"errors"
	"slices"

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
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/mappers"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/set"
)

// SchematicConfigurationControllerName is the name of the SchematicConfiguration controller.
const SchematicConfigurationControllerName = "SchematicConfigurationController"

// SchematicConfigurationController combines MachineExtensions resource, MachineStatus overlay into SchematicConfiguration for each existing ClusterMachine.
// Ensures schematic exists in the image factory.
type SchematicConfigurationController = qtransform.QController[*omni.ClusterMachine, *omni.SchematicConfiguration]

// NewSchematicConfigurationController initializes SchematicConfigurationController.
func NewSchematicConfigurationController(imageFactoryClient ImageFactoryClient) *SchematicConfigurationController {
	helper := &schematicConfigurationHelper{
		imageFactoryClient: imageFactoryClient,
	}

	return qtransform.NewQController(
		qtransform.Settings[*omni.ClusterMachine, *omni.SchematicConfiguration]{
			Name: SchematicConfigurationControllerName,
			MapMetadataFunc: func(clusterMachine *omni.ClusterMachine) *omni.SchematicConfiguration {
				return omni.NewSchematicConfiguration(resources.DefaultNamespace, clusterMachine.Metadata().ID())
			},
			UnmapMetadataFunc: func(schematicConfiguration *omni.SchematicConfiguration) *omni.ClusterMachine {
				return omni.NewClusterMachine(resources.DefaultNamespace, schematicConfiguration.Metadata().ID())
			},
			TransformExtraOutputFunc: func(ctx context.Context, r controller.ReaderWriter, _ *zap.Logger,
				clusterMachine *omni.ClusterMachine, schematicConfiguration *omni.SchematicConfiguration,
			) error {
				status, err := helper.reconcile(ctx, r, clusterMachine, schematicConfiguration)
				if err != nil {
					return err
				}

				return safe.WriterModify(ctx, r, status, func(r *omni.MachineExtensionsStatus) error {
					helpers.CopyAllLabels(status, r)

					r.TypedSpec().Value = status.TypedSpec().Value

					return nil
				})
			},
			FinalizerRemovalExtraOutputFunc: func(ctx context.Context, r controller.ReaderWriter, _ *zap.Logger, clusterMachine *omni.ClusterMachine) error {
				statusMD := omni.NewMachineExtensionsStatus(resources.DefaultNamespace, clusterMachine.Metadata().ID()).Metadata()

				ready, err := helpers.TeardownAndDestroy(ctx, r, statusMD)
				if err != nil {
					return err
				}

				if !ready {
					return nil
				}

				err = r.RemoveFinalizer(ctx, omni.NewMachineExtensions(resources.DefaultNamespace, clusterMachine.Metadata().ID()).Metadata(), SchematicConfigurationControllerName)
				if err != nil && !state.IsNotFoundError(err) {
					return err
				}

				return nil
			},
		},
		qtransform.WithExtraMappedInput[*omni.MachineStatus](
			qtransform.MapperSameID[*omni.ClusterMachine](),
		),
		qtransform.WithExtraMappedInput[*omni.Cluster](
			mappers.MapClusterResourceToLabeledResources[*omni.ClusterMachine](),
		),
		qtransform.WithExtraMappedInput[*omni.Schematic](
			qtransform.MapperNone(),
		),
		qtransform.WithExtraMappedInput[*omni.MachineExtensions](
			qtransform.MapperSameID[*omni.ClusterMachine](),
		),
		qtransform.WithExtraMappedInput[*omni.KernelArgs](
			qtransform.MapperSameID[*omni.ClusterMachine](),
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
//nolint:gocognit
func (helper *schematicConfigurationHelper) reconcile(
	ctx context.Context,
	r controller.ReaderWriter,
	clusterMachine *omni.ClusterMachine,
	schematicConfiguration *omni.SchematicConfiguration,
) (*omni.MachineExtensionsStatus, error) {
	extensions, err := safe.ReaderGetByID[*omni.MachineExtensions](ctx, r, clusterMachine.Metadata().ID())
	if err != nil && !state.IsNotFoundError(err) {
		return nil, err
	}

	if extensions != nil {
		if err = updateFinalizers(ctx, r, extensions); err != nil {
			return nil, err
		}
	}

	clusterName, ok := clusterMachine.Metadata().Labels().Get(omni.LabelCluster)
	if !ok {
		return nil, errors.New("failed to determine cluster")
	}

	var (
		overlay                 schematic.Overlay
		machineExtensionsStatus = omni.NewMachineExtensionsStatus(resources.DefaultNamespace, clusterMachine.Metadata().ID())
	)

	helpers.CopyLabels(clusterMachine, machineExtensionsStatus, omni.LabelCluster, omni.LabelWorkerRole, omni.LabelControlPlaneRole, omni.LabelMachineSet)

	ms, err := safe.ReaderGet[*omni.MachineStatus](ctx, r, omni.NewMachineStatus(resources.DefaultNamespace, clusterMachine.Metadata().ID()).Metadata())
	if err != nil {
		if state.IsNotFoundError(err) {
			return machineExtensionsStatus, nil
		}

		return nil, err
	}

	machineExtensionsStatus.TypedSpec().Value.TalosVersion = ms.TypedSpec().Value.TalosVersion

	securityState := ms.TypedSpec().Value.SecurityState
	if securityState == nil {
		return nil, xerrors.NewTaggedf[qtransform.SkipReconcileTag]("boot type for machine %q is not yet set", ms.Metadata().ID())
	}

	schematicInitialized := ms.TypedSpec().Value.Schematic != nil

	if schematicInitialized {
		overlay = getOverlay(ms)
	}

	cluster, err := safe.ReaderGetByID[*omni.Cluster](ctx, r, clusterName)
	if err != nil {
		return nil, err
	}

	kernelArgs, err := determineKernelArgs(ctx, ms, r)
	if err != nil {
		return nil, err
	}

	customization, err := newMachineCustomization(cluster, ms, extensions, securityState.BootedWithUki)
	if err != nil {
		return nil, err
	}

	schematicConfiguration.TypedSpec().Value.TalosVersion = cluster.TypedSpec().Value.TalosVersion

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

	if _, err = safe.ReaderGetByID[*omni.Schematic](ctx, r, id); err != nil {
		if !state.IsNotFoundError(err) {
			return nil, err
		}

		if _, err = helper.imageFactoryClient.EnsureSchematic(ctx, config); err != nil {
			return nil, err
		}
	}

	schematicConfiguration.TypedSpec().Value.SchematicId = id

	helpers.CopyLabels(clusterMachine, schematicConfiguration, omni.LabelCluster)

	machineExtensionsStatus.TypedSpec().Value.Extensions = computeMachineExtensionsStatus(ms, &customization)

	return machineExtensionsStatus, nil
}

func getOverlay(ms *omni.MachineStatus) schematic.Overlay {
	if ms.TypedSpec().Value.Schematic.Overlay == nil {
		return schematic.Overlay{}
	}

	overlay := ms.TypedSpec().Value.Schematic.Overlay

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

func newMachineCustomization(cluster *omni.Cluster, machineStatus *omni.MachineStatus, exts *omni.MachineExtensions, isUKI bool) (machineCustomization, error) {
	mc := machineCustomization{
		machineStatus: machineStatus,
		cluster:       cluster,
	}

	if isUKI {
		schematicConfig := machineStatus.TypedSpec().Value.Schematic
		if schematicConfig == nil {
			return mc, xerrors.NewTaggedf[qtransform.SkipReconcileTag]("machine is booted with UKI but the schematic information is not yet available")
		}

		mc.meta = xslices.Map(schematicConfig.MetaValues,
			func(v *specs.MetaValue) schematic.MetaValue {
				return schematic.MetaValue{
					Key:   uint8(v.GetKey()),
					Value: v.GetValue(),
				}
			})
	}

	extensionsExplicitlyDefined := exts != nil && exts.Metadata().Phase() == resource.PhaseRunning
	if extensionsExplicitlyDefined {
		mc.machineExtensions = exts
	}

	detected, err := getDetectedExtensions(cluster, machineStatus)
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

	revertToInitialState := !extensionsExplicitlyDefined && len(mc.extensionsList) == 0 && !machineStatus.TypedSpec().Value.Schematic.GetInAgentMode()
	if revertToInitialState { // extensions are not explicitly set, we should revert them to their initial state
		initialState := machineStatus.TypedSpec().Value.Schematic.GetInitialState()
		if initialState == nil {
			return mc, xerrors.NewTaggedf[qtransform.SkipReconcileTag]("machine initial schematic state is not yet set")
		}

		mc.extensionsList = initialState.Extensions
	}

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
}

func determineKernelArgs(ctx context.Context, machineStatus *omni.MachineStatus, r controller.Reader) ([]string, error) {
	if _, kernelArgsInitialized := machineStatus.Metadata().Annotations().Get(omni.KernelArgsInitialized); !kernelArgsInitialized {
		return nil, xerrors.NewTaggedf[qtransform.SkipReconcileTag]("kernel args are not yet initialized")
	}

	updateSupported, err := kernelargs.UpdateSupported(machineStatus)
	if err != nil {
		return nil, err
	}

	if !updateSupported {
		if machineStatus.TypedSpec().Value.Schematic == nil {
			return nil, xerrors.NewTaggedf[qtransform.SkipReconcileTag]("machine schematic is not yet initialized")
		}

		// keep them unchanged - take the current args as-is
		return machineStatus.TypedSpec().Value.Schematic.KernelArgs, nil
	}

	kernelArgs, err := kernelargs.GetUncached(ctx, r, machineStatus.Metadata().ID())
	if err != nil && !state.IsNotFoundError(err) {
		return nil, err
	}

	// todo: centralize this...
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

	if ms.TypedSpec().Value.Schematic != nil {
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
