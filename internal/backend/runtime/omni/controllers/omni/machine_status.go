// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"fmt"
	"strings"

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
	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
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
			{
				Namespace: resources.InfraProviderNamespace,
				Type:      infra.InfraMachineStatusType,
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

		return nil, err
	}

	switch ptr.Type() {
	case omni.MachineStatusType,
		omni.MachineType,
		omni.MachineSetNodeType,
		omni.ClusterMachineStatusType,
		omni.MachineLabelsType,
		infra.InfraMachineStatusType,
		omni.MachineStatusSnapshotType:
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

		maintenanceMode = inputs.machineStatusSnapshot.TypedSpec().Value.MachineStatus.GetStage() == machineapi.MachineStatusEvent_MAINTENANCE
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

		helpers.CopyLabels(machine, m, omni.LabelMachineRequest, omni.LabelMachineRequestSet, omni.LabelNoManualAllocation, omni.LabelIsManagedByStaticInfraProvider)

		infraProvider, ok := machine.Metadata().Annotations().Get(omni.LabelInfraProviderID)
		if ok {
			m.Metadata().Labels().Set(omni.LabelInfraProviderID, infraProvider)
		} else {
			m.Metadata().Labels().Delete(omni.LabelInfraProviderID)
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

		if err = ctrl.setClusterRelation(inputs, m); err != nil {
			return err
		}

		ctrl.updateMachineConnectionStatus(machine, inputs, m)
		ctrl.updateMachinePowerState(machine, inputs, m)

		return ctrl.copyInfraProviderLabels(m, inputs.infraMachineStatus)
	})
}

func (ctrl *MachineStatusController) copyInfraProviderLabels(machineStatus *omni.MachineStatus, infraMachineStatus *infra.MachineStatus) error {
	if infraMachineStatus == nil {
		return nil
	}

	providerID, ok := infraMachineStatus.Metadata().Labels().Get(omni.LabelInfraProviderID)
	if !ok {
		return fmt.Errorf("missing %q label on infra machine status", omni.LabelInfraProviderID)
	}

	labelPrefix := fmt.Sprintf(omni.InfraProviderLabelPrefixFormat, providerID)

	// remove all existing provider ID labels
	for k := range machineStatus.Metadata().Labels().Raw() {
		if strings.HasPrefix(k, labelPrefix) {
			machineStatus.Metadata().Labels().Delete(k)
		}
	}

	for k, v := range infraMachineStatus.Metadata().Labels().Raw() {
		if strings.HasPrefix(k, omni.SystemLabelPrefix) { // skip system labels
			continue
		}

		machineStatus.Metadata().Labels().Set(labelPrefix+k, v)
	}

	return nil
}

func (ctrl *MachineStatusController) updateMachinePowerState(machine *omni.Machine, inputs inputs, m *omni.MachineStatus) {
	_, isManagedByStaticInfraProvider := machine.Metadata().Labels().Get(omni.LabelIsManagedByStaticInfraProvider)

	if isManagedByStaticInfraProvider {
		if inputs.infraMachineStatus == nil {
			m.TypedSpec().Value.PowerState = specs.MachineStatusSpec_POWER_STATE_UNKNOWN

			return
		}

		switch inputs.infraMachineStatus.TypedSpec().Value.PowerState {
		case specs.InfraMachineStatusSpec_POWER_STATE_UNKNOWN:
			m.TypedSpec().Value.PowerState = specs.MachineStatusSpec_POWER_STATE_UNKNOWN
		case specs.InfraMachineStatusSpec_POWER_STATE_ON:
			m.TypedSpec().Value.PowerState = specs.MachineStatusSpec_POWER_STATE_ON
		case specs.InfraMachineStatusSpec_POWER_STATE_OFF:
			m.TypedSpec().Value.PowerState = specs.MachineStatusSpec_POWER_STATE_OFF
		}

		return
	}

	m.TypedSpec().Value.PowerState = specs.MachineStatusSpec_POWER_STATE_UNSUPPORTED
}

func (ctrl *MachineStatusController) updateMachineConnectionStatus(machine *omni.Machine, inputs inputs, m *omni.MachineStatus) {
	connected := machine.TypedSpec().Value.Connected

	m.TypedSpec().Value.Connected = connected

	if connected {
		m.Metadata().Labels().Set(omni.MachineStatusLabelConnected, "")
		m.Metadata().Labels().Delete(omni.MachineStatusLabelDisconnected)

		m.Metadata().Labels().Set(omni.MachineStatusLabelReadyToUse, "")

		return
	}

	m.Metadata().Labels().Delete(omni.MachineStatusLabelConnected)

	infraMachineIsReadyToUse := inputs.infraMachineStatus != nil &&
		inputs.infraMachineStatus.TypedSpec().Value.PowerState != specs.InfraMachineStatusSpec_POWER_STATE_UNKNOWN &&
		inputs.infraMachineStatus.TypedSpec().Value.ReadyToUse

	_, isManagedByStaticInfraProvider := machine.Metadata().Labels().Get(omni.LabelIsManagedByStaticInfraProvider)
	if isManagedByStaticInfraProvider && infraMachineIsReadyToUse && m.TypedSpec().Value.Cluster == "" {
		m.Metadata().Labels().Set(omni.MachineStatusLabelReadyToUse, "")
		m.Metadata().Labels().Delete(omni.MachineStatusLabelDisconnected)

		return
	}

	m.Metadata().Labels().Delete(omni.MachineStatusLabelReadyToUse)
	m.Metadata().Labels().Set(omni.MachineStatusLabelDisconnected, "")
}

func (ctrl *MachineStatusController) reconcileTearingDown(ctx context.Context, r controller.QRuntime, logger *zap.Logger, machine *omni.Machine) error {
	ctrl.runner.StopTask(logger, machine.Metadata().ID())

	_, err := ctrl.handleInputs(ctx, r, machine)
	if err != nil {
		return err
	}

	md := omni.NewMachineStatus(resources.DefaultNamespace, machine.Metadata().ID()).Metadata()

	ready, err := helpers.TeardownAndDestroy(ctx, r, md)
	if err != nil {
		return err
	}

	if !ready {
		return nil
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

		if event.Diagnostics != nil {
			spec.Diagnostics = event.Diagnostics
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

		if event.SecurityState != nil {
			spec.SecurityState = event.SecurityState
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

			spec.Schematic.InAgentMode = event.Schematic.InAgentMode
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
	infraMachineStatus    *infra.MachineStatus
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

	in.infraMachineStatus, err = helpers.HandleInput[*infra.MachineStatus](ctx, r, ctrl.Name(), machine)
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
