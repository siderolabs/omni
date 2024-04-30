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
	"github.com/siderolabs/image-factory/pkg/schematic"
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/imagefactory"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/mappers"
)

// SchematicConfigurationControllerName is the name of the SchematicConfiguration controller.
const SchematicConfigurationControllerName = "SchematicConfigurationController"

// SchematicConfigurationController combines MachineExtensions resource, MachineStatus overlay into SchematicConfiguration for each existing ClusterMachine.
// Ensures schematic exists in the image factory.
type SchematicConfigurationController = qtransform.QController[*omni.ClusterMachine, *omni.SchematicConfiguration]

// NewSchematicConfigurationController initializes SchematicConfigurationController.
func NewSchematicConfigurationController(imageFactoryClient *imagefactory.Client) *SchematicConfigurationController {
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
			TransformExtraOutputFunc: helper.reconcile,
			FinalizerRemovalExtraOutputFunc: func(ctx context.Context, r controller.ReaderWriter, _ *zap.Logger, clusterMachine *omni.ClusterMachine) error {
				err := r.RemoveFinalizer(ctx, omni.NewMachineExtensions(resources.DefaultNamespace, clusterMachine.Metadata().ID()).Metadata(), SchematicConfigurationControllerName)
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
	)
}

type schematicConfigurationHelper struct {
	imageFactoryClient *imagefactory.Client
}

// Reconcile implements controller.QController interface.
func (helper *schematicConfigurationHelper) reconcile(
	ctx context.Context,
	r controller.ReaderWriter,
	_ *zap.Logger,
	clusterMachine *omni.ClusterMachine,
	schematicConfiguration *omni.SchematicConfiguration,
) error {
	extensions, err := safe.ReaderGetByID[*omni.MachineExtensions](ctx, r, clusterMachine.Metadata().ID())
	if err != nil && !state.IsNotFoundError(err) {
		return err
	}

	if extensions != nil {
		if err = updateFinalizers(ctx, r, extensions); err != nil {
			return err
		}
	}

	clusterName, ok := clusterMachine.Metadata().Labels().Get(omni.LabelCluster)
	if !ok {
		return errors.New("failed to determine cluster")
	}

	var (
		overlay          schematic.Overlay
		initialSchematic string
		currentSchematic string
	)

	ms, err := safe.ReaderGet[*omni.MachineStatus](ctx, r, omni.NewMachineStatus(resources.DefaultNamespace, clusterMachine.Metadata().ID()).Metadata())
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil
		}

		return err
	}

	if ms.TypedSpec().Value.Schematic != nil {
		overlay = getOverlay(ms)

		initialSchematic = ms.TypedSpec().Value.Schematic.InitialSchematic
		currentSchematic = ms.TypedSpec().Value.Schematic.Id
	}

	cluster, err := safe.ReaderGetByID[*omni.Cluster](ctx, r, clusterName)
	if err != nil {
		return err
	}

	machineExtensions, err := newMachineExtensions(cluster, ms, extensions)
	if err != nil {
		return err
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

		return nil
	}

	config := schematic.Schematic{
		Customization: schematic.Customization{
			SystemExtensions: schematic.SystemExtensions{
				OfficialExtensions: machineExtensions.extensionsList,
			},
		},
		Overlay: overlay,
	}

	id, err := config.ID()
	if err != nil {
		return err
	}

	if _, err := safe.ReaderGetByID[*omni.Schematic](ctx, r, id); err != nil {
		if !state.IsNotFoundError(err) {
			return err
		}

		if _, err = helper.imageFactoryClient.EnsureSchematic(ctx, config); err != nil {
			return err
		}
	}

	schematicConfiguration.TypedSpec().Value.SchematicId = id

	helpers.CopyLabels(clusterMachine, schematicConfiguration, omni.LabelCluster)

	return nil
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
	machineStatus     *omni.MachineStatus
	machineExtensions *omni.MachineExtensions
	cluster           *omni.Cluster
	extensionsList    []string
}

func newMachineExtensions(cluster *omni.Cluster, machineStatus *omni.MachineStatus, extensions *omni.MachineExtensions) (machineExtensions, error) {
	me := machineExtensions{
		machineStatus: machineStatus,
		cluster:       cluster,
	}

	if extensions != nil && extensions.Metadata().Phase() == resource.PhaseRunning {
		me.machineExtensions = extensions
	}

	detected, err := getDetectedExtensions(cluster, machineStatus)
	if err != nil {
		return me, err
	}

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

func getDetectedExtensions(cluster *omni.Cluster, machineStatus *omni.MachineStatus) ([]string, error) {
	if machineStatus.TypedSpec().Value.InitialTalosVersion == "" {
		return nil, errors.New("machine initial version is not set")
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
