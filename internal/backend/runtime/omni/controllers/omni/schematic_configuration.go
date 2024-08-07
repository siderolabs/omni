// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"errors"
	"slices"

	"github.com/blang/semver"
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xerrors"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/image-factory/pkg/schematic"
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
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
func NewSchematicConfigurationController(imageFactoryClient SchematicEnsurer) *SchematicConfigurationController {
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

				ready, err := r.Teardown(ctx, statusMD)
				if err != nil && !state.IsNotFoundError(err) {
					return err
				}

				if ready {
					if err = r.Destroy(ctx, statusMD); err != nil && !state.IsNotFoundError(err) {
						return err
					}
				}

				err = r.RemoveFinalizer(ctx, omni.NewMachineExtensions(resources.DefaultNamespace, clusterMachine.Metadata().ID()).Metadata(), SchematicConfigurationControllerName)
				if err != nil && !state.IsNotFoundError(err) {
					return err
				}

				return nil
			},
		},
		qtransform.WithExtraMappedInput(
			qtransform.MapperSameID[*omni.MachineStatus, *omni.ClusterMachine](),
		),
		qtransform.WithExtraMappedInput(
			mappers.MapClusterResourceToLabeledResources[*omni.Cluster, *omni.ClusterMachine](),
		),
		qtransform.WithExtraMappedInput(
			qtransform.MapperNone[*omni.Schematic](),
		),
		qtransform.WithExtraMappedInput(
			qtransform.MapperSameID[*omni.MachineExtensions, *omni.ClusterMachine](),
		),
		qtransform.WithExtraMappedInput(
			qtransform.MapperNone[*siderolink.ConnectionParams](),
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
	imageFactoryClient SchematicEnsurer
}

// Reconcile implements controller.QController interface.
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
		initialSchematic        string
		currentSchematic        string
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

	secureBootStatus := ms.TypedSpec().Value.SecureBootStatus
	if secureBootStatus == nil {
		return nil, xerrors.NewTaggedf[qtransform.SkipReconcileTag]("secure boot status for machine %q is not yet set", ms.Metadata().ID())
	}

	schematicInitialized := ms.TypedSpec().Value.Schematic != nil

	if schematicInitialized {
		overlay = getOverlay(ms)
		initialSchematic = ms.TypedSpec().Value.Schematic.InitialSchematic

		if secureBootStatus.Enabled {
			currentSchematic = ms.TypedSpec().Value.Schematic.FullId
		} else {
			currentSchematic = ms.TypedSpec().Value.Schematic.Id
		}
	}

	cluster, err := safe.ReaderGetByID[*omni.Cluster](ctx, r, clusterName)
	if err != nil {
		return nil, err
	}

	machineExtensions, err := newMachineExtensions(cluster, ms, extensions, secureBootStatus.Enabled)
	if err != nil {
		return nil, err
	}

	schematicConfiguration.TypedSpec().Value.TalosVersion = cluster.TypedSpec().Value.TalosVersion

	if !shouldGenerateSchematicID(cluster, machineExtensions, ms, overlay) {
		// if extensions config is not set, fall back to the initial schematic id and exit
		id := initialSchematic

		if id == "" {
			id = currentSchematic
		}

		schematicConfiguration.TypedSpec().Value.SchematicId = id
		helpers.CopyLabels(clusterMachine, schematicConfiguration, omni.LabelCluster)

		if schematicInitialized {
			machineExtensionsStatus.TypedSpec().Value.Extensions = computeMachineExtensionsStatus(ms, nil)
		}

		return machineExtensionsStatus, nil
	}

	config := schematic.Schematic{
		Customization: schematic.Customization{
			SystemExtensions: schematic.SystemExtensions{
				OfficialExtensions: machineExtensions.extensionsList,
			},
			ExtraKernelArgs: machineExtensions.kernelArgs,
			Meta:            machineExtensions.meta,
		},
		Overlay: overlay,
	}

	id, err := config.ID()
	if err != nil {
		return nil, err
	}

	if _, err := safe.ReaderGetByID[*omni.Schematic](ctx, r, id); err != nil {
		if !state.IsNotFoundError(err) {
			return nil, err
		}

		if _, err = helper.imageFactoryClient.EnsureSchematic(ctx, config); err != nil {
			return nil, err
		}
	}

	schematicConfiguration.TypedSpec().Value.SchematicId = id

	helpers.CopyLabels(clusterMachine, schematicConfiguration, omni.LabelCluster)

	machineExtensionsStatus.TypedSpec().Value.Extensions = computeMachineExtensionsStatus(ms, &machineExtensions)

	return machineExtensionsStatus, nil
}

func shouldGenerateSchematicID(cluster *omni.Cluster, extensionsList machineExtensions, machineStatus *omni.MachineStatus, overlay schematic.Overlay) bool {
	// migrating SBC running Talos < 1.7.0, overlay was detected, but is not applied yet, should generate a schematic
	if overlay.Name != "" &&
		!quirks.New(machineStatus.TypedSpec().Value.InitialTalosVersion).SupportsOverlay() &&
		quirks.New(cluster.TypedSpec().Value.TalosVersion).SupportsOverlay() {
		return true
	}

	return extensionsList.shouldGenerateSchematic()
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

type machineExtensions struct {
	machineStatus      *omni.MachineStatus
	machineExtensions  *omni.MachineExtensions
	cluster            *omni.Cluster
	extensionsList     []string
	detectedExtensions []string
	kernelArgs         []string
	meta               []schematic.MetaValue
}

func newMachineExtensions(cluster *omni.Cluster, machineStatus *omni.MachineStatus, extensions *omni.MachineExtensions, secureBootEnabled bool) (machineExtensions, error) {
	me := machineExtensions{
		machineStatus: machineStatus,
		cluster:       cluster,
	}

	if secureBootEnabled {
		schematicConfig := machineStatus.TypedSpec().Value.Schematic
		if schematicConfig == nil {
			return me, xerrors.NewTaggedf[qtransform.SkipReconcileTag]("secure boot is enabled but the schematic information is not yet available")
		}

		me.kernelArgs = schematicConfig.KernelArgs
		me.meta = xslices.Map(schematicConfig.MetaValues,
			func(v *specs.MetaValue) schematic.MetaValue {
				return schematic.MetaValue{
					Key:   uint8(v.GetKey()),
					Value: v.GetValue(),
				}
			})
	}

	if extensions != nil && extensions.Metadata().Phase() == resource.PhaseRunning {
		me.machineExtensions = extensions
	}

	detected, err := getDetectedExtensions(cluster, machineStatus)
	if err != nil {
		return me, err
	}

	me.detectedExtensions = detected

	me.extensionsList = append(me.extensionsList, detected...)

	if me.machineExtensions != nil {
		for _, e := range me.machineExtensions.TypedSpec().Value.Extensions {
			if slices.Contains(me.extensionsList, e) {
				continue
			}

			me.extensionsList = append(me.extensionsList, e)
		}
	}

	slices.Sort(me.extensionsList)

	return me, nil
}

func (m machineExtensions) shouldGenerateSchematic() bool {
	// generate schematic for the machine extensions when either machine extensions exists
	// and contains the explicit empty list for the schematics, or when schematic list is not empty
	return m.machineExtensions != nil || len(m.extensionsList) != 0
}

func (m *machineExtensions) isDetected(value string) bool {
	return slices.Contains(m.detectedExtensions, value)
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
			"siderolabs/bnx2-bnx2x",
			"siderolabs/intel-ice-firmware",
		}, nil
	}

	return nil, nil
}

func computeMachineExtensionsStatus(ms *omni.MachineStatus, me *machineExtensions) []*specs.MachineExtensionsStatusSpec_Item {
	var (
		installedExtensions set.Set[string]
		requestedExtensions set.Set[string]
	)

	if ms.TypedSpec().Value.Schematic != nil {
		installedExtensions = xslices.ToSet(ms.TypedSpec().Value.Schematic.Extensions)
	}

	if me != nil {
		requestedExtensions = xslices.ToSet(me.extensionsList)
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

		if me != nil {
			switch {
			// detected extensions should always be pre installed
			case me.isDetected(extension):
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
