// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

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
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/extensions"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/mappers"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/set"
)

// SchematicConfigurationControllerName is the name of the SchematicConfiguration controller.
const SchematicConfigurationControllerName = "SchematicConfigurationController"

// SchematicConfigurationController combines MachineExtensions resource, MachineStatus overlay into SchematicConfiguration for each existing ClusterMachine.
// Ensures schematic exists in the image factory.
type SchematicConfigurationController struct {
	*qtransform.QController[*omni.ClusterMachine, *omni.SchematicConfiguration]
	imageFactoryClient SchematicEnsurer
}

// NewSchematicConfigurationController initializes SchematicConfigurationController.
func NewSchematicConfigurationController(imageFactoryClient SchematicEnsurer) *SchematicConfigurationController {
	ctrl := &SchematicConfigurationController{
		imageFactoryClient: imageFactoryClient,
	}

	ctrl.QController = qtransform.NewQController(
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
				extsStatus, argsStatus, err := ctrl.reconcile(ctx, r, clusterMachine, schematicConfiguration)
				if err != nil {
					return err
				}

				if err = safe.WriterModify(ctx, r, extsStatus, func(r *omni.MachineExtensionsStatus) error {
					helpers.CopyAllLabels(extsStatus, r)

					r.TypedSpec().Value = extsStatus.TypedSpec().Value

					return nil
				}); err != nil {
					return fmt.Errorf("failed to update machine extensions status: %w", err)
				}

				if err = safe.WriterModify(ctx, r, argsStatus, func(r *omni.MachineExtraKernelArgsStatus) error {
					helpers.CopyAllLabels(argsStatus, r)

					r.TypedSpec().Value = argsStatus.TypedSpec().Value

					return nil
				}); err != nil {
					return fmt.Errorf("failed to update machine extra kernel args status: %w", err)
				}

				return nil
			},
			FinalizerRemovalExtraOutputFunc: func(ctx context.Context, r controller.ReaderWriter, _ *zap.Logger, clusterMachine *omni.ClusterMachine) error {
				extsReady, err := helpers.TeardownAndDestroy(ctx, r, omni.NewMachineExtensionsStatus(resources.DefaultNamespace, clusterMachine.Metadata().ID()).Metadata())
				if err != nil {
					return err
				}

				if extsReady {
					extsMD := omni.NewMachineExtensions(resources.DefaultNamespace, clusterMachine.Metadata().ID()).Metadata()

					if err = r.RemoveFinalizer(ctx, extsMD, SchematicConfigurationControllerName); err != nil && !state.IsNotFoundError(err) {
						return err
					}
				}

				argsReady, err := helpers.TeardownAndDestroy(ctx, r, omni.NewMachineExtraKernelArgsStatus(resources.DefaultNamespace, clusterMachine.Metadata().ID()).Metadata())
				if err != nil {
					return err
				}

				if argsReady {
					argsMD := omni.NewMachineExtraKernelArgs(resources.DefaultNamespace, clusterMachine.Metadata().ID()).Metadata()

					if err = r.RemoveFinalizer(ctx, argsMD, SchematicConfigurationControllerName); err != nil && !state.IsNotFoundError(err) {
						return err
					}
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
		qtransform.WithExtraMappedInput[*omni.MachineExtraKernelArgs](
			qtransform.MapperSameID[*omni.ClusterMachine](),
		),
		qtransform.WithExtraOutputs(
			controller.Output{
				Type: omni.MachineExtensionsStatusType,
				Kind: controller.OutputExclusive,
			},
			controller.Output{
				Type: omni.MachineExtraKernelArgsStatusType,
				Kind: controller.OutputExclusive,
			},
		),
	)

	return ctrl
}

// Reconcile implements controller.QController interface.
func (ctrl *SchematicConfigurationController) reconcile(ctx context.Context, r controller.ReaderWriter,
	clusterMachine *omni.ClusterMachine, schematicConfiguration *omni.SchematicConfiguration,
) (*omni.MachineExtensionsStatus, *omni.MachineExtraKernelArgsStatus, error) {
	extensions, extraKernelArgs, err := ctrl.getCustomizationResources(ctx, r, clusterMachine.Metadata().ID())
	if err != nil {
		return nil, nil, err
	}

	clusterName, ok := clusterMachine.Metadata().Labels().Get(omni.LabelCluster)
	if !ok {
		return nil, nil, errors.New("failed to determine cluster")
	}

	var (
		overlay                      schematic.Overlay
		initialSchematic             string
		currentSchematic             string
		machineExtensionsStatus      = omni.NewMachineExtensionsStatus(resources.DefaultNamespace, clusterMachine.Metadata().ID())
		machineExtraKernelArgsStatus = omni.NewMachineExtraKernelArgsStatus(resources.DefaultNamespace, clusterMachine.Metadata().ID())
	)

	helpers.CopyLabels(clusterMachine, machineExtensionsStatus, omni.LabelCluster, omni.LabelWorkerRole, omni.LabelControlPlaneRole, omni.LabelMachineSet)
	helpers.CopyLabels(clusterMachine, machineExtraKernelArgsStatus, omni.LabelCluster, omni.LabelWorkerRole, omni.LabelControlPlaneRole, omni.LabelMachineSet)

	ms, err := safe.ReaderGet[*omni.MachineStatus](ctx, r, omni.NewMachineStatus(resources.DefaultNamespace, clusterMachine.Metadata().ID()).Metadata())
	if err != nil {
		if state.IsNotFoundError(err) {
			return machineExtensionsStatus, machineExtraKernelArgsStatus, nil
		}

		return nil, nil, err
	}

	machineExtensionsStatus.TypedSpec().Value.TalosVersion = ms.TypedSpec().Value.TalosVersion

	securityState := ms.TypedSpec().Value.SecurityState
	if securityState == nil {
		return nil, nil, xerrors.NewTaggedf[qtransform.SkipReconcileTag]("boot type for machine %q is not yet set", ms.Metadata().ID())
	}

	schematicInitialized := ms.TypedSpec().Value.Schematic != nil
	isUKI := securityState.BootedWithUki

	if isUKI && !schematicInitialized {
		return nil, nil, xerrors.NewTaggedf[qtransform.SkipReconcileTag]("machine %q is booted with UKI but the schematic information is not yet available", ms.Metadata().ID())
	}

	if schematicInitialized {
		overlay = ctrl.getOverlay(ms)
		initialSchematic = ms.TypedSpec().Value.Schematic.InitialSchematic

		if isUKI {
			currentSchematic = ms.TypedSpec().Value.Schematic.FullId
		} else {
			currentSchematic = ms.TypedSpec().Value.Schematic.Id
		}
	}

	cluster, err := safe.ReaderGetByID[*omni.Cluster](ctx, r, clusterName)
	if err != nil {
		return nil, nil, err
	}

	mc, err := ctrl.newMachineCustomization(cluster, ms, extensions, extraKernelArgs)
	if err != nil {
		return nil, nil, err
	}

	schematicConfiguration.TypedSpec().Value.TalosVersion = cluster.TypedSpec().Value.TalosVersion

	if !ctrl.shouldGenerateSchematicID(cluster, mc, ms, overlay) {
		// if extensions config is not set, fall back to the initial schematic id and exit
		id := initialSchematic

		if id == "" {
			id = currentSchematic
		}

		schematicConfiguration.TypedSpec().Value.SchematicId = id
		helpers.CopyLabels(clusterMachine, schematicConfiguration, omni.LabelCluster)

		if schematicInitialized {
			machineExtensionsStatus.TypedSpec().Value.Extensions = ctrl.computeMachineExtensionsStatus(ms, nil)
			machineExtraKernelArgsStatus.TypedSpec().Value = ctrl.computeExtraKernelArgsStatus(ms, nil)
		}

		return machineExtensionsStatus, machineExtraKernelArgsStatus, nil
	}

	config := schematic.Schematic{
		Customization: schematic.Customization{
			SystemExtensions: schematic.SystemExtensions{
				OfficialExtensions: mc.extensions,
			},
			ExtraKernelArgs: mc.kernelArgs,
			Meta:            mc.meta,
		},
		Overlay: overlay,
	}

	id, err := config.ID()
	if err != nil {
		return nil, nil, err
	}

	if _, err = safe.ReaderGetByID[*omni.Schematic](ctx, r, id); err != nil {
		if !state.IsNotFoundError(err) {
			return nil, nil, err
		}

		if _, err = ctrl.imageFactoryClient.EnsureSchematic(ctx, config); err != nil {
			return nil, nil, err
		}
	}

	schematicConfiguration.TypedSpec().Value.SchematicId = id

	helpers.CopyLabels(clusterMachine, schematicConfiguration, omni.LabelCluster)

	machineExtensionsStatus.TypedSpec().Value.Extensions = ctrl.computeMachineExtensionsStatus(ms, &mc)
	machineExtraKernelArgsStatus.TypedSpec().Value = ctrl.computeExtraKernelArgsStatus(ms, &mc)

	return machineExtensionsStatus, machineExtraKernelArgsStatus, nil
}

func (ctrl *SchematicConfigurationController) getCustomizationResources(ctx context.Context,
	r controller.ReaderWriter, id resource.ID,
) (*omni.MachineExtensions, *omni.MachineExtraKernelArgs, error) {
	extensions, err := safe.ReaderGetByID[*omni.MachineExtensions](ctx, r, id)
	if err != nil && !state.IsNotFoundError(err) {
		return nil, nil, err
	}

	extraKernelArgs, err := safe.ReaderGetByID[*omni.MachineExtraKernelArgs](ctx, r, id)
	if err != nil && !state.IsNotFoundError(err) {
		return nil, nil, err
	}

	if extensions != nil {
		if err = ctrl.updateFinalizers(ctx, r, extensions.Metadata()); err != nil {
			return nil, nil, err
		}
	}

	if extraKernelArgs != nil {
		if err = ctrl.updateFinalizers(ctx, r, extraKernelArgs.Metadata()); err != nil {
			return nil, nil, err
		}
	}

	return extensions, extraKernelArgs, nil
}

func (ctrl *SchematicConfigurationController) shouldGenerateSchematicID(cluster *omni.Cluster, me machineCustomization, machineStatus *omni.MachineStatus, overlay schematic.Overlay) bool {
	// migrating SBC running Talos < 1.7.0, overlay was detected, but is not applied yet, should generate a schematic
	if overlay.Name != "" &&
		!quirks.New(machineStatus.TypedSpec().Value.InitialTalosVersion).SupportsOverlay() &&
		quirks.New(cluster.TypedSpec().Value.TalosVersion).SupportsOverlay() {
		return true
	}

	return me.shouldGenerateSchematic
}

func (ctrl *SchematicConfigurationController) getOverlay(ms *omni.MachineStatus) schematic.Overlay {
	if ms.TypedSpec().Value.Schematic.Overlay == nil {
		return schematic.Overlay{}
	}

	overlay := ms.TypedSpec().Value.Schematic.Overlay

	return schematic.Overlay{
		Name:  overlay.Name,
		Image: overlay.Image,
	}
}

func (ctrl *SchematicConfigurationController) updateFinalizers(ctx context.Context, r controller.ReaderWriter, md *resource.Metadata) error {
	if md.Phase() == resource.PhaseTearingDown {
		return r.RemoveFinalizer(ctx, md, SchematicConfigurationControllerName)
	}

	if md.Finalizers().Has(SchematicConfigurationControllerName) {
		return nil
	}

	return r.AddFinalizer(ctx, md, SchematicConfigurationControllerName)
}

type machineCustomization struct {
	cluster                 *omni.Cluster
	extensions              []string
	detectedExtensions      []string
	kernelArgs              []string
	meta                    []schematic.MetaValue
	shouldGenerateSchematic bool
}

func (ctrl *SchematicConfigurationController) newMachineCustomization(cluster *omni.Cluster, ms *omni.MachineStatus,
	exts *omni.MachineExtensions, args *omni.MachineExtraKernelArgs,
) (machineCustomization, error) {
	me := machineCustomization{
		cluster: cluster,
	}

	explicitExtensions := exts != nil && exts.Metadata().Phase() == resource.PhaseRunning
	explicitKernelArgs := args != nil && args.Metadata().Phase() == resource.PhaseRunning

	schematicConfig := ms.TypedSpec().Value.Schematic
	if schematicConfig != nil {
		me.extensions = ctrl.determineBaseExtensions(ms)
		me.kernelArgs = ctrl.determineKernelArgs(ms, explicitKernelArgs)
		me.meta = xslices.Map(schematicConfig.MetaValues,
			func(v *specs.MetaValue) schematic.MetaValue {
				return schematic.MetaValue{
					Key:   uint8(v.GetKey()),
					Value: v.GetValue(),
				}
			})
	}

	if explicitExtensions {
		me.shouldGenerateSchematic = true

		me.extensions = exts.TypedSpec().Value.Extensions
	}

	if explicitKernelArgs {
		me.shouldGenerateSchematic = true

		me.kernelArgs = append(me.kernelArgs, args.TypedSpec().Value.Args...)
	}

	detected, err := ctrl.getDetectedExtensions(cluster, ms)
	if err != nil {
		return machineCustomization{}, err
	}

	me.detectedExtensions = detected

	me.extensions = append(me.extensions, detected...)

	clusterVersion, err := semver.ParseTolerant(cluster.TypedSpec().Value.TalosVersion)
	if err != nil {
		return me, err
	}

	me.extensions = extensions.MapNamesByVersion(me.extensions, clusterVersion)

	slices.Sort(me.extensions)
	me.extensions = slices.Compact(me.extensions)

	if len(me.extensions) != 0 {
		me.shouldGenerateSchematic = true
	}

	return me, nil
}

// determineKernelArgs determines the kernel args for a machine on a best-effort basis.
func (ctrl *SchematicConfigurationController) determineKernelArgs(ms *omni.MachineStatus, explicit bool) []string {
	var args []string

	if ms.TypedSpec().Value.Schematic.InitialState != nil {
		args = ms.TypedSpec().Value.Schematic.InitialState.KernelArgs
	} else {
		args = ms.TypedSpec().Value.Schematic.KernelArgs
	}

	if explicit {
		return xslices.Filter(args, ctrl.isProtectedKernelArg)
	}

	return args
}

func (ctrl *SchematicConfigurationController) isProtectedKernelArg(arg string) bool {
	for _, prefix := range []string{
		constants.KernelParamSideroLink, constants.KernelParamEventsSink, constants.KernelParamLoggingKernel,
		constants.KernelParamConfig, constants.KernelParamConfigEarly, constants.KernelParamConfigInline,
	} {
		if strings.HasPrefix(arg, prefix+"=") {
			return true
		}
	}

	return false
}

// determineBaseExtensions determines the base extensions for a machine on a best-effort basis.
func (ctrl *SchematicConfigurationController) determineBaseExtensions(ms *omni.MachineStatus) []string {
	// use the initial set of extensions if available
	if ms.TypedSpec().Value.Schematic.InitialState != nil {
		return ms.TypedSpec().Value.Schematic.InitialState.Extensions
	}

	// otherwise, use current extensions - we have no way of knowing what was initially installed
	return ms.TypedSpec().Value.Schematic.Extensions
}

func (m *machineCustomization) isDetected(value string) bool {
	return slices.Contains(m.detectedExtensions, value)
}

func (ctrl *SchematicConfigurationController) getDetectedExtensions(cluster *omni.Cluster, machineStatus *omni.MachineStatus) ([]string, error) {
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

	return ctrl.detectExtensions(initialVersion, currentVersion)
}

func (ctrl *SchematicConfigurationController) detectExtensions(initialVersion, currentVersion semver.Version) ([]string, error) {
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

func (ctrl *SchematicConfigurationController) computeMachineExtensionsStatus(ms *omni.MachineStatus, mc *machineCustomization) []*specs.MachineExtensionsStatusSpec_Item {
	var (
		installedExtensions set.Set[string]
		requestedExtensions set.Set[string]
	)

	if ms.TypedSpec().Value.Schematic != nil {
		installedExtensions = xslices.ToSet(ms.TypedSpec().Value.Schematic.Extensions)
		// Here, we default to the currently installed extensions.
		// If we know the requested extensions (either they are explicitly set, or we know the initial set of extensions from the machine status), we will set them below,
		// so that we can show the correct status. but if we don't know the requested extensions, i.e., they are not explicitly set, and we don't have them in the machine status,
		// we cannot know if it changed or not, so we assume that it didn't change.
		requestedExtensions = xslices.ToSet(ms.TypedSpec().Value.Schematic.Extensions)

		if ms.TypedSpec().Value.Schematic.InitialState != nil {
			requestedExtensions = xslices.ToSet(ms.TypedSpec().Value.Schematic.InitialState.Extensions)
		}
	}

	if mc != nil {
		requestedExtensions = xslices.ToSet(mc.extensions)
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

		switch {
		// detected extensions should always be pre installed
		case mc != nil && mc.isDetected(extension):
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

func (ctrl *SchematicConfigurationController) computeExtraKernelArgsStatus(ms *omni.MachineStatus, mc *machineCustomization) *specs.MachineExtraKernelArgsStatusSpec {
	var (
		currentKernelArgs   []string
		requestedKernelArgs []string
	)

	if ms.TypedSpec().Value.Schematic != nil {
		currentKernelArgs = ms.TypedSpec().Value.Schematic.KernelArgs
		// Here, we default to the current kernel args.
		// If we know the requested kernel args (either they are explicitly set, or we know the initial set of kernel args from the machine status), we will set them below,
		// so that we can show the correct status. but if we don't know the requested kernel args, i.e., they are not explicitly set, and we don't have them in the machine status,
		// we cannot know if it changed or not, so we assume that it didn't change.
		requestedKernelArgs = currentKernelArgs

		if ms.TypedSpec().Value.Schematic.InitialState != nil {
			requestedKernelArgs = ms.TypedSpec().Value.Schematic.InitialState.KernelArgs
		}
	}

	if mc != nil {
		requestedKernelArgs = mc.kernelArgs
	}

	phase := specs.MachineExtraKernelArgsStatusSpec_Configured
	if !slices.Equal(currentKernelArgs, requestedKernelArgs) {
		phase = specs.MachineExtraKernelArgsStatusSpec_Configuring
	}

	return &specs.MachineExtraKernelArgsStatusSpec{
		Args:  requestedKernelArgs,
		Phase: phase,
	}
}
