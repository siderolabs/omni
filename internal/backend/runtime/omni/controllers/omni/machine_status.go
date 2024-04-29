// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/kvutils"
	"github.com/cosi-project/runtime/pkg/safe"
	cosistate "github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/image-factory/pkg/schematic"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/imagefactory"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/task"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/task/machine"
)

// SchematicEnsurer ensures that the given schematic exists in the image factory.
type SchematicEnsurer interface {
	EnsureSchematic(ctx context.Context, inputSchematic schematic.Schematic) (imagefactory.EnsuredSchematic, error)
}

// MachineStatusController manages omni.MachineStatuses based on information from Talos API.
type MachineStatusController struct {
	runner             *task.Runner[machine.InfoChan, machine.CollectTaskSpec]
	ImageFactoryClient SchematicEnsurer
}

// Name implements controller.Controller interface.
func (ctrl *MachineStatusController) Name() string {
	return "MachineStatusController"
}

// Inputs implements controller.Controller interface.
func (ctrl *MachineStatusController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: resources.DefaultNamespace,
			Type:      omni.MachineStatusType,
			Kind:      controller.InputDestroyReady,
		},
		{
			Namespace: resources.DefaultNamespace,
			Type:      omni.MachineType,
			Kind:      controller.InputStrong,
		},
		{
			Namespace: resources.DefaultNamespace,
			Type:      omni.ClusterMachineType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: resources.DefaultNamespace,
			Type:      omni.TalosConfigType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: resources.DefaultNamespace,
			Type:      omni.MachineStatusSnapshotType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: resources.DefaultNamespace,
			Type:      omni.MachineLabelsType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *MachineStatusController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: omni.MachineStatusType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *MachineStatusController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	ctrl.runner = task.NewEqualRunner[machine.CollectTaskSpec]()
	defer ctrl.runner.Stop()

	notifyCh := make(chan machine.Info)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
			if err := ctrl.reconcileCollectors(ctx, r, logger, notifyCh); err != nil {
				return err
			}
		case event := <-notifyCh:
			if err := ctrl.handleNotification(ctx, r, event); err != nil {
				return err
			}
		}

		r.ResetRestartBackoff()
	}
}

//nolint:gocognit,gocyclo,cyclop
func (ctrl *MachineStatusController) reconcileCollectors(ctx context.Context, r controller.Runtime, logger *zap.Logger, notifyCh machine.InfoChan) error {
	// list machines
	list, err := safe.ReaderListAll[*omni.Machine](ctx, r)
	if err != nil {
		return fmt.Errorf("error listing machines: %w", err)
	}

	tracker := trackResource(r, resources.DefaultNamespace, omni.MachineStatusType)

	tracker.beforeDestroyCallback = func(res resource.Resource) error {
		return r.RemoveFinalizer(ctx, omni.NewMachine(resources.DefaultNamespace, res.Metadata().ID()).Metadata(), ctrl.Name())
	}

	// figure out which collectors should run
	shouldRun := map[string]machine.CollectTaskSpec{}
	machines := map[resource.ID]*omni.Machine{}
	machineLabels := map[resource.ID]*omni.MachineLabels{}
	reportingEvents := map[string]struct{}{}

	for iter := list.Iterator(); iter.Next(); {
		item := iter.Value()
		machineSpec := item.TypedSpec().Value

		var labels *omni.MachineLabels

		labels, err = safe.ReaderGet[*omni.MachineLabels](ctx, r,
			resource.NewMetadata(resources.DefaultNamespace, omni.MachineLabelsType, item.Metadata().ID(), resource.VersionUndefined))
		if err != nil {
			if !cosistate.IsNotFoundError(err) {
				return err
			}
		}

		machineLabels[item.Metadata().ID()] = labels

		if item.Metadata().Phase() == resource.PhaseTearingDown {
			continue
		}

		if !item.Metadata().Finalizers().Has(ctrl.Name()) {
			if err = r.AddFinalizer(ctx, item.Metadata(), ctrl.Name()); err != nil {
				return err
			}
		}

		tracker.keep(item)

		var clusterMachine resource.Resource

		clusterMachine, err = r.Get(ctx, resource.NewMetadata(resources.DefaultNamespace, omni.ClusterMachineType, item.Metadata().ID(), resource.VersionUndefined))
		if err != nil && !cosistate.IsNotFoundError(err) {
			return err
		}

		var cluster string

		if clusterMachine != nil {
			var ok bool
			cluster, ok = clusterMachine.Metadata().Labels().Get(omni.LabelCluster)

			if !ok {
				logger.Warn("The cluster machine is created without the cluster")
			}
		}

		var talosConfig *omni.TalosConfig

		if cluster != "" {
			talosConfig, err = safe.ReaderGet[*omni.TalosConfig](ctx, r, resource.NewMetadata(
				resources.DefaultNamespace, omni.TalosConfigType, cluster, resource.VersionUndefined,
			))

			if err != nil && !cosistate.IsNotFoundError(err) {
				return err
			}
		}

		var machineStatusSnapshot *omni.MachineStatusSnapshot
		machineStatusSnapshot, err = safe.ReaderGet[*omni.MachineStatusSnapshot](ctx, r, resource.NewMetadata(
			resources.DefaultNamespace,
			omni.MachineStatusSnapshotType,
			item.Metadata().ID(),
			resource.VersionUndefined,
		))

		if err != nil && !cosistate.IsNotFoundError(err) {
			return err
		}

		var maintenanceStage bool

		if machineStatusSnapshot != nil {
			maintenanceStage = machineStatusSnapshot.TypedSpec().Value.MachineStatus.Stage == machineapi.MachineStatusEvent_MAINTENANCE
			reportingEvents[item.Metadata().ID()] = struct{}{}
		}

		if machineSpec.Connected {
			shouldRun[item.Metadata().ID()] = machine.CollectTaskSpec{
				Endpoint:        machineSpec.ManagementAddress,
				TalosConfig:     talosConfig,
				MaintenanceMode: talosConfig == nil || maintenanceStage,
				MachineID:       item.Metadata().ID(),
				MachineLabels:   labels,
			}
		}

		machines[item.Metadata().ID()] = item
	}

	ctrl.runner.Reconcile(ctx, logger, shouldRun, notifyCh)

	if err = tracker.cleanup(ctx); err != nil {
		return err
	}

	for id := range machines {
		if err = safe.WriterModify(ctx, r, omni.NewMachineStatus(resources.DefaultNamespace, id), func(m *omni.MachineStatus) error {
			spec := m.TypedSpec().Value

			connected := machines[id].TypedSpec().Value.Connected

			spec.Connected = connected

			if connected {
				m.Metadata().Labels().Set(omni.MachineStatusLabelConnected, "")
				m.Metadata().Labels().Delete(omni.MachineStatusLabelDisconnected)
			} else {
				m.Metadata().Labels().Delete(omni.MachineStatusLabelConnected)
				m.Metadata().Labels().Set(omni.MachineStatusLabelDisconnected, "")
			}

			if _, ok := reportingEvents[id]; ok {
				m.Metadata().Labels().Set(omni.MachineStatusLabelReportingEvents, "")
			} else {
				m.Metadata().Labels().Delete(omni.MachineStatusLabelReportingEvents)
			}

			if shouldRun[id].Endpoint != "" {
				spec.ManagementAddress = shouldRun[id].Endpoint
			}

			var clusterMachine *omni.ClusterMachine

			clusterMachine, err = safe.ReaderGet[*omni.ClusterMachine](ctx, r, resource.NewMetadata(resources.DefaultNamespace, omni.ClusterMachineType, id, resource.VersionUndefined))
			if err != nil {
				if !cosistate.IsNotFoundError(err) {
					return err
				}
			}

			helpers.CopyUserLabels(m, ctrl.mergeLabels(m, machineLabels[m.Metadata().ID()]))

			omni.MachineStatusReconcileLabels(m)

			return ctrl.setClusterRelation(clusterMachine, m)
		}); err != nil && !cosistate.IsPhaseConflictError(err) {
			return err
		}
	}

	return nil
}

func (ctrl *MachineStatusController) mergeLabels(m *omni.MachineStatus, machineLabels *omni.MachineLabels) map[string]string {
	labels := map[string]string{}

	if m.TypedSpec().Value.ImageLabels != nil {
		for k, v := range m.TypedSpec().Value.ImageLabels {
			labels[k] = v
		}
	}

	if machineLabels != nil {
		for k, v := range machineLabels.Metadata().Labels().Raw() {
			labels[k] = v
		}
	}

	return labels
}

func (ctrl *MachineStatusController) setClusterRelation(clusterMachine *omni.ClusterMachine, machineStatus *omni.MachineStatus) error {
	if clusterMachine == nil {
		machineStatus.TypedSpec().Value.Cluster = ""
		machineStatus.TypedSpec().Value.Role = specs.MachineStatusSpec_NONE

		machineStatus.Metadata().Labels().Set(omni.MachineStatusLabelAvailable, "")

		machineStatus.Metadata().Labels().Delete(omni.LabelCluster)

		return nil
	}

	labels := clusterMachine.Metadata().Labels()

	cluster, clusterOk := labels.Get(omni.LabelCluster)
	if !clusterOk {
		return fmt.Errorf("malformed ClusterMachine resource: no %q label, cluster ownership unknown", omni.LabelCluster)
	}

	machineStatus.TypedSpec().Value.Cluster = cluster

	_, controlPlane := labels.Get(omni.LabelControlPlaneRole)
	_, worker := labels.Get(omni.LabelWorkerRole)

	machineStatus.Metadata().Labels().Set(omni.LabelCluster, cluster)

	switch {
	case controlPlane:
		machineStatus.TypedSpec().Value.Role = specs.MachineStatusSpec_CONTROL_PLANE
	case worker:
		machineStatus.TypedSpec().Value.Role = specs.MachineStatusSpec_WORKER
	default:
		return fmt.Errorf("malformed ClusterMachine resource: no %q or %q label, role unknown", omni.LabelControlPlaneRole, omni.LabelWorkerRole)
	}

	machineStatus.Metadata().Labels().Delete(omni.MachineStatusLabelAvailable)

	return nil
}

//nolint:gocognit,gocyclo,cyclop
func (ctrl *MachineStatusController) handleNotification(ctx context.Context, r controller.Runtime, event machine.Info) error {
	if err := safe.WriterModify(ctx, r, omni.NewMachineStatus(resources.DefaultNamespace, event.MachineID), func(m *omni.MachineStatus) error {
		spec := m.TypedSpec().Value

		if event.LastError != nil {
			spec.LastError = event.LastError.Error()
		} else {
			spec.LastError = ""
		}

		if event.TalosVersion != nil {
			spec.TalosVersion = *event.TalosVersion

			if spec.InitialTalosVersion == "" {
				spec.InitialTalosVersion = spec.TalosVersion
			}
		}

		if spec.Network == nil {
			spec.Network = &specs.MachineStatusSpec_NetworkStatus{}
		}

		if event.Hostname != nil {
			spec.Network.Hostname = *event.Hostname
		}

		if event.Domainname != nil {
			spec.Network.Domainname = *event.Domainname
		}

		if event.Addresses != nil {
			spec.Network.Addresses = event.Addresses
		}

		if event.DefaultGateways != nil {
			spec.Network.DefaultGateways = event.DefaultGateways
		}

		if event.NetworkLinks != nil {
			spec.Network.NetworkLinks = event.NetworkLinks
		}

		if spec.Hardware == nil {
			spec.Hardware = &specs.MachineStatusSpec_HardwareStatus{}
		}

		if event.Arch != nil {
			spec.Hardware.Arch = *event.Arch
		}

		if event.Processors != nil {
			spec.Hardware.Processors = event.Processors
		}

		if event.MemoryModules != nil {
			spec.Hardware.MemoryModules = event.MemoryModules
		}

		if event.Blockdevices != nil {
			spec.Hardware.Blockdevices = event.Blockdevices
		}

		if event.PlatformMetadata != nil {
			spec.PlatformMetadata = event.PlatformMetadata
		}

		if event.ImageLabels != nil {
			spec.ImageLabels = event.ImageLabels

			m.Metadata().Labels().Do(func(temp kvutils.TempKV) {
				for key, value := range spec.ImageLabels {
					temp.Set(key, value)
				}
			})
		}

		if event.NoAccess {
			m.Metadata().Labels().Set(omni.MachineStatusLabelInvalidState, "")
		} else {
			m.Metadata().Labels().Delete(omni.MachineStatusLabelInvalidState)
		}

		if event.Schematic != nil {
			if spec.Schematic == nil {
				spec.Schematic = &specs.MachineStatusSpec_Schematic{}
			}

			spec.Schematic.Extensions = event.Schematic.Extensions
			spec.Schematic.Id = event.Schematic.Id
			spec.Schematic.Invalid = event.Schematic.Invalid
			if event.Schematic.Overlay != nil && event.Schematic.Overlay.Name != "" {
				spec.Schematic.Overlay = event.Schematic.Overlay
			}

			if spec.Schematic.Invalid {
				spec.Schematic.InitialSchematic = ""
			} else if spec.Schematic.InitialSchematic == "" {
				spec.Schematic.InitialSchematic = spec.Schematic.Id
			}
		}

		if event.MaintenanceConfig != nil { // never set the maintenance config back to nil if it was ever initialized
			spec.MaintenanceConfig = event.MaintenanceConfig
		}

		spec.Maintenance = event.MaintenanceMode

		omni.MachineStatusReconcileLabels(m)

		return nil
	}); err != nil && !cosistate.IsPhaseConflictError(err) {
		return fmt.Errorf("error modifying resource: %w", err)
	}

	if event.Schematic != nil && event.Schematic.Id != "" {
		machineSchematic := schematic.Schematic{
			Customization: schematic.Customization{
				SystemExtensions: schematic.SystemExtensions{
					OfficialExtensions: event.Schematic.Extensions,
				},
			},
		}

		if _, err := ctrl.ImageFactoryClient.EnsureSchematic(ctx, machineSchematic); err != nil {
			return fmt.Errorf("error ensuring schematic: %w", err)
		}
	}

	return nil
}
