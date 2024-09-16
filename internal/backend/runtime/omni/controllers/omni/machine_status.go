// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/kvutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/task"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/image-factory/pkg/schematic"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/internal/backend/imagefactory"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
	machinetask "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/task/machine"
)

// SchematicEnsurer ensures that the given schematic exists in the image factory.
type SchematicEnsurer interface {
	EnsureSchematic(ctx context.Context, inputSchematic schematic.Schematic) (imagefactory.EnsuredSchematic, error)
}

// MachineStatusControllerName is the name of the MachineStatusController.
const MachineStatusControllerName = "MachineStatusController"

// MachineStatusController manages omni.MachineStatuses based on information from Talos API.
type MachineStatusController struct {
	ImageFactoryClient SchematicEnsurer
	runner             *task.Runner[machinetask.InfoChan, machinetask.CollectTaskSpec]
	notifyCh           chan machinetask.Info
	generic.NamedController
}

// NewMachineStatusController initializes MachineStatusController.
func NewMachineStatusController(imageFactoryClient SchematicEnsurer) *MachineStatusController {
	return &MachineStatusController{
		NamedController: generic.NamedController{
			ControllerName: MachineStatusControllerName,
		},
		notifyCh:           make(chan machinetask.Info),
		runner:             task.NewEqualRunner[machinetask.CollectTaskSpec](),
		ImageFactoryClient: imageFactoryClient,
	}
}

// Settings implements controller.QController interface.
func (ctrl *MachineStatusController) Settings() controller.QSettings {
	return controller.QSettings{
		Inputs: []controller.Input{
			{
				Namespace: resources.DefaultNamespace,
				Type:      omni.MachineType,
				Kind:      controller.InputQPrimary,
			},
			{
				Namespace: resources.DefaultNamespace,
				Type:      omni.MachineStatusType,
				Kind:      controller.InputQMappedDestroyReady,
			},
			{
				Namespace: resources.DefaultNamespace,
				Type:      omni.MachineSetNodeType,
				Kind:      controller.InputQMapped,
			},
			{
				Namespace: resources.DefaultNamespace,
				Type:      omni.ClusterMachineStatusType,
				Kind:      controller.InputQMapped,
			},
			{
				Namespace: resources.DefaultNamespace,
				Type:      omni.TalosConfigType,
				Kind:      controller.InputQMapped,
			},
			{
				Namespace: resources.DefaultNamespace,
				Type:      omni.MachineStatusSnapshotType,
				Kind:      controller.InputQMapped,
			},
			{
				Namespace: resources.DefaultNamespace,
				Type:      omni.MachineLabelsType,
				Kind:      controller.InputQMapped,
			},
			{
				Namespace: resources.DefaultNamespace,
				Type:      siderolink.ConnectionParamsType,
				Kind:      controller.InputQMapped,
			},
		},
		Outputs: []controller.Output{
			{
				Kind: controller.OutputExclusive,
				Type: omni.MachineStatusType,
			},
		},
		Concurrency: optional.Some[uint](4),
		RunHook: func(ctx context.Context, _ *zap.Logger, r controller.QRuntime) error {
			for {
				select {
				case <-ctx.Done():
					return nil
				case event := <-ctrl.notifyCh:
					if err := ctrl.handleNotification(ctx, r, event); err != nil {
						return err
					}
				}
			}
		},
		ShutdownHook: func() {
			ctrl.runner.Stop()
		},
	}
}

// MapInput implements controller.QController interface.
func (ctrl *MachineStatusController) MapInput(ctx context.Context, _ *zap.Logger,
	r controller.QRuntime, ptr resource.Pointer,
) ([]resource.Pointer, error) {
	_, err := r.Get(ctx, ptr)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, nil
		}
	}

	switch ptr.Type() {
	case omni.MachineStatusType:
		fallthrough
	case omni.MachineType:
		fallthrough
	case omni.MachineSetNodeType:
		fallthrough
	case omni.ClusterMachineStatusType:
		fallthrough
	case omni.MachineLabelsType:
		fallthrough
	case omni.MachineStatusSnapshotType:
		return []resource.Pointer{
			omni.NewMachine(resources.DefaultNamespace, ptr.ID()).Metadata(),
		}, nil
	case omni.TalosConfigType:
		machines, err := safe.ReaderListAll[*omni.ClusterMachineStatus](ctx, r, state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, ptr.ID())))
		if err != nil {
			return nil, err
		}

		res := make([]resource.Pointer, 0, machines.Len())

		machines.ForEach(func(r *omni.ClusterMachineStatus) {
			res = append(res, omni.NewMachine(resources.DefaultNamespace, r.Metadata().ID()).Metadata())
		})

		return res, nil
	case siderolink.ConnectionParamsType:
		return nil, nil
	}

	return nil, fmt.Errorf("unexpected resource type %q", ptr.Type())
}

// Reconcile implements controller.QController interface.
func (ctrl *MachineStatusController) Reconcile(ctx context.Context,
	logger *zap.Logger, r controller.QRuntime, ptr resource.Pointer,
) error {
	machine, err := safe.ReaderGet[*omni.Machine](ctx, r, omni.NewMachine(ptr.Namespace(), ptr.ID()).Metadata())
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil
		}

		return err
	}

	if machine.Metadata().Phase() == resource.PhaseTearingDown {
		return ctrl.reconcileTearingDown(ctx, r, logger, machine)
	}

	return ctrl.reconcileRunning(ctx, r, logger, machine)
}

func (ctrl *MachineStatusController) reconcileRunning(ctx context.Context, r controller.QRuntime, logger *zap.Logger, machine *omni.Machine) error {
	if !machine.Metadata().Finalizers().Has(ctrl.Name()) {
		if err := r.AddFinalizer(ctx, machine.Metadata(), ctrl.Name()); err != nil {
			return err
		}
	}

	inputs, err := ctrl.handleInputs(ctx, r, machine)
	if err != nil {
		return err
	}

	var (
		reportingEvents bool
		maintenanceMode bool
	)

	if inputs.machineStatusSnapshot != nil {
		reportingEvents = true

		maintenanceMode = inputs.machineStatusSnapshot.TypedSpec().Value.MachineStatus.Stage == machineapi.MachineStatusEvent_MAINTENANCE
	}

	var params *siderolink.ConnectionParams

	params, err = safe.ReaderGetByID[*siderolink.ConnectionParams](ctx, r, siderolink.ConfigID)
	if err != nil {
		return fmt.Errorf("error reading connection params: %w", err)
	}

	spec := machinetask.CollectTaskSpec{
		Endpoint:                   machine.TypedSpec().Value.ManagementAddress,
		TalosConfig:                inputs.talosConfig,
		MaintenanceMode:            inputs.talosConfig == nil || maintenanceMode,
		MachineID:                  machine.Metadata().ID(),
		MachineLabels:              inputs.machineLabels,
		DefaultSchematicKernelArgs: siderolink.KernelArgs(params),
	}

	if !machine.TypedSpec().Value.Connected {
		ctrl.runner.StopTask(logger, machine.Metadata().ID())
	}

	if machine.TypedSpec().Value.Connected {
		ctrl.runner.StartTask(ctx, logger, machine.Metadata().ID(), spec, ctrl.notifyCh)
	}

	return safe.WriterModify(ctx, r, omni.NewMachineStatus(resources.DefaultNamespace, machine.Metadata().ID()), func(m *omni.MachineStatus) error {
		spec := m.TypedSpec().Value

		connected := machine.TypedSpec().Value.Connected

		spec.Connected = connected

		helpers.CopyLabels(machine, m, omni.LabelMachineRequest, omni.LabelMachineRequestSet)

		if connected {
			m.Metadata().Labels().Set(omni.MachineStatusLabelConnected, "")
			m.Metadata().Labels().Delete(omni.MachineStatusLabelDisconnected)
		} else {
			m.Metadata().Labels().Delete(omni.MachineStatusLabelConnected)
			m.Metadata().Labels().Set(omni.MachineStatusLabelDisconnected, "")
		}

		if reportingEvents {
			m.Metadata().Labels().Set(omni.MachineStatusLabelReportingEvents, "")
		} else {
			m.Metadata().Labels().Delete(omni.MachineStatusLabelReportingEvents)
		}

		if machine.TypedSpec().Value.ManagementAddress != "" {
			spec.ManagementAddress = machine.TypedSpec().Value.ManagementAddress
		}

		helpers.CopyUserLabels(m, ctrl.mergeLabels(m, inputs.machineLabels))

		omni.MachineStatusReconcileLabels(m)

		return ctrl.setClusterRelation(inputs, m)
	})
}

func (ctrl *MachineStatusController) reconcileTearingDown(ctx context.Context, r controller.QRuntime, logger *zap.Logger, machine *omni.Machine) error {
	ctrl.runner.StopTask(logger, machine.Metadata().ID())

	_, err := ctrl.handleInputs(ctx, r, machine)
	if err != nil {
		return err
	}

	md := omni.NewMachineStatus(resources.DefaultNamespace, machine.Metadata().ID()).Metadata()

	ready, err := r.Teardown(ctx, md)
	if err != nil {
		if state.IsNotFoundError(err) {
			return r.RemoveFinalizer(ctx, machine.Metadata(), ctrl.Name())
		}

		return err
	}

	if !ready {
		return nil
	}

	if err = r.Destroy(ctx, md); err != nil {
		return err
	}

	return r.RemoveFinalizer(ctx, machine.Metadata(), ctrl.Name())
}

//nolint:gocyclo,cyclop,gocognit
func (ctrl *MachineStatusController) handleNotification(ctx context.Context, r controller.QRuntime, event machinetask.Info) error {
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

		if event.SecureBootStatus != nil {
			spec.SecureBootStatus = event.SecureBootStatus
		}

		if event.Schematic != nil {
			if spec.Schematic == nil {
				spec.Schematic = &specs.MachineStatusSpec_Schematic{}
			}

			spec.Schematic.Id = event.Schematic.ID
			spec.Schematic.FullId = event.Schematic.FullID
			spec.Schematic.Extensions = event.Schematic.Extensions
			spec.Schematic.Invalid = event.Schematic.Invalid
			spec.Schematic.KernelArgs = event.Schematic.KernelArgs
			spec.Schematic.MetaValues = xslices.Map(event.Schematic.MetaValues, func(value schematic.MetaValue) *specs.MetaValue {
				return &specs.MetaValue{
					Key:   uint32(value.Key),
					Value: value.Value,
				}
			})

			if event.Schematic.Overlay.Name != "" {
				spec.Schematic.Overlay = &specs.Overlay{
					Name:  event.Schematic.Overlay.Name,
					Image: event.Schematic.Overlay.Image,
				}
			}

			if spec.Schematic.Invalid {
				spec.Schematic.InitialSchematic = ""
			} else if spec.Schematic.InitialSchematic == "" {
				spec.Schematic.InitialSchematic = spec.Schematic.FullId
			}
		}

		spec.Maintenance = event.MaintenanceMode

		omni.MachineStatusReconcileLabels(m)

		return nil
	}); err != nil && !state.IsPhaseConflictError(err) {
		return fmt.Errorf("error modifying resource: %w", err)
	}

	if event.Schematic != nil && event.Schematic.ID != "" {
		machineSchematic := schematic.Schematic{
			Customization: schematic.Customization{
				SystemExtensions: schematic.SystemExtensions{
					OfficialExtensions: event.Schematic.Extensions,
				},
				ExtraKernelArgs: event.Schematic.KernelArgs,
				Meta:            event.Schematic.MetaValues,
			},
			Overlay: event.Schematic.Overlay,
		}

		if _, err := ctrl.ImageFactoryClient.EnsureSchematic(ctx, machineSchematic); err != nil {
			return fmt.Errorf("error ensuring schematic: %w", err)
		}
	}

	return nil
}

type inputs struct {
	clusterMachineStatus  *omni.ClusterMachineStatus
	machineSetNode        *omni.MachineSetNode
	machineLabels         *omni.MachineLabels
	machineStatusSnapshot *omni.MachineStatusSnapshot
	talosConfig           *omni.TalosConfig
}

func (ctrl *MachineStatusController) handleInputs(ctx context.Context, r controller.QRuntime, machine *omni.Machine) (inputs, error) {
	var (
		in  inputs
		err error
	)

	in.machineStatusSnapshot, err = helpers.HandleInput[*omni.MachineStatusSnapshot](ctx, r, ctrl.Name(), machine)
	if err != nil {
		return in, err
	}

	in.machineLabels, err = helpers.HandleInput[*omni.MachineLabels](ctx, r, ctrl.Name(), machine)
	if err != nil {
		return in, err
	}

	in.clusterMachineStatus, err = helpers.HandleInput[*omni.ClusterMachineStatus](ctx, r, ctrl.Name(), machine)
	if err != nil {
		return in, err
	}

	in.machineSetNode, err = helpers.HandleInput[*omni.MachineSetNode](ctx, r, ctrl.Name(), machine)
	if err != nil {
		return in, err
	}

	if in.machineSetNode != nil {
		clusterName, ok := in.machineSetNode.Metadata().Labels().Get(omni.LabelCluster)
		if ok {
			in.talosConfig, err = safe.ReaderGetByID[*omni.TalosConfig](ctx, r, clusterName)
			if err != nil && !state.IsNotFoundError(err) {
				return in, err
			}
		}
	}

	return in, nil
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

func (ctrl *MachineStatusController) setClusterRelation(in inputs, machineStatus *omni.MachineStatus) error {
	var md *resource.Metadata

	switch {
	case in.clusterMachineStatus != nil:
		md = in.clusterMachineStatus.Metadata()
	case in.machineSetNode != nil:
		md = in.machineSetNode.Metadata()
	}

	if md == nil {
		machineStatus.TypedSpec().Value.Cluster = ""
		machineStatus.TypedSpec().Value.Role = specs.MachineStatusSpec_NONE

		machineStatus.Metadata().Labels().Set(omni.MachineStatusLabelAvailable, "")

		machineStatus.Metadata().Labels().Delete(omni.LabelCluster)
		machineStatus.Metadata().Labels().Delete(omni.LabelControlPlaneRole)
		machineStatus.Metadata().Labels().Delete(omni.LabelWorkerRole)

		return nil
	}

	labels := md.Labels()

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
		machineStatus.Metadata().Labels().Set(omni.LabelControlPlaneRole, "")
		machineStatus.Metadata().Labels().Delete(omni.LabelWorkerRole)
	case worker:
		machineStatus.TypedSpec().Value.Role = specs.MachineStatusSpec_WORKER
		machineStatus.Metadata().Labels().Set(omni.LabelWorkerRole, "")
		machineStatus.Metadata().Labels().Delete(omni.LabelControlPlaneRole)
	default:
		return fmt.Errorf("malformed ClusterMachine resource: no %q or %q label, role unknown", omni.LabelControlPlaneRole, omni.LabelWorkerRole)
	}

	machineStatus.Metadata().Labels().Delete(omni.MachineStatusLabelAvailable)

	return nil
}
