// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"fmt"
	"maps"
	"net/http"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/kvutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/task"
	"github.com/siderolabs/gen/optional"
	factoryclient "github.com/siderolabs/image-factory/pkg/client"
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
	machinectrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/machine"
)

type KernelArgsInitializer interface {
	Init(ctx context.Context, id resource.ID, args []string) error
}

// MachineStatusControllerName is the name of the MachineStatusController.
const MachineStatusControllerName = "MachineStatusController"

// MachineStatusController manages omni.MachineStatuses based on information from Talos API.
type MachineStatusController struct {
	ImageFactoryClient    ImageFactoryClient
	kernelArgsInitializer KernelArgsInitializer
	runner                *task.Runner[machinetask.InfoChan, machinetask.CollectTaskSpec]
	notifyCh              chan machinetask.Info
	generic.NamedController
}

// NewMachineStatusController initializes MachineStatusController.
func NewMachineStatusController(imageFactoryClient ImageFactoryClient, kernelArgsInitializer KernelArgsInitializer) *MachineStatusController {
	return &MachineStatusController{
		NamedController: generic.NamedController{
			ControllerName: MachineStatusControllerName,
		},
		notifyCh:              make(chan machinetask.Info),
		runner:                task.NewEqualRunner[machinetask.CollectTaskSpec](),
		ImageFactoryClient:    imageFactoryClient,
		kernelArgsInitializer: kernelArgsInitializer,
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
				Type:      siderolink.MachineJoinConfigType,
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
			{
				Kind: controller.OutputShared,
				Type: omni.MachineLabelsType,
			},
		},
		Concurrency: optional.Some[uint](4),
		RunHook: func(ctx context.Context, logger *zap.Logger, r controller.QRuntime) error {
			for {
				select {
				case <-ctx.Done():
					return nil
				case event := <-ctrl.notifyCh:
					if err := ctrl.handleNotification(ctx, r, event, logger); err != nil {
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
	r controller.QRuntime, ptr controller.ReducedResourceMetadata,
) ([]resource.Pointer, error) {
	switch ptr.Type() {
	case omni.MachineStatusType,
		omni.MachineType,
		omni.MachineSetNodeType,
		omni.ClusterMachineStatusType,
		omni.MachineLabelsType,
		infra.InfraMachineStatusType,
		omni.MachineStatusSnapshotType,
		siderolink.MachineJoinConfigType:
		return []resource.Pointer{
			omni.NewMachine(ptr.ID()).Metadata(),
		}, nil
	case omni.TalosConfigType:
		machines, err := safe.ReaderListAll[*omni.ClusterMachineStatus](ctx, r, state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, ptr.ID())))
		if err != nil {
			return nil, err
		}

		res := make([]resource.Pointer, 0, machines.Len())

		machines.ForEach(func(r *omni.ClusterMachineStatus) {
			res = append(res, omni.NewMachine(r.Metadata().ID()).Metadata())
		})

		return res, nil
	}

	return nil, fmt.Errorf("unexpected resource type %q", ptr.Type())
}

// Reconcile implements controller.QController interface.
func (ctrl *MachineStatusController) Reconcile(ctx context.Context,
	logger *zap.Logger, r controller.QRuntime, ptr resource.Pointer,
) error {
	machine, err := safe.ReaderGet[*omni.Machine](ctx, r, omni.NewMachine(ptr.ID()).Metadata())
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

	var config *siderolink.MachineJoinConfig

	config, err = safe.ReaderGetByID[*siderolink.MachineJoinConfig](ctx, r, machine.Metadata().ID())
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil
		}

		return err
	}

	spec := machinetask.CollectTaskSpec{
		Endpoint:                   machine.TypedSpec().Value.ManagementAddress,
		TalosConfig:                inputs.talosConfig,
		MaintenanceMode:            inputs.talosConfig == nil || maintenanceMode,
		MachineID:                  machine.Metadata().ID(),
		MachineLabels:              inputs.machineLabels,
		InfraMachineStatus:         inputs.infraMachineStatus,
		DefaultSchematicKernelArgs: config.TypedSpec().Value.Config.KernelArgs,
	}

	if !machine.TypedSpec().Value.Connected {
		ctrl.runner.StopTask(logger, machine.Metadata().ID())
	}

	if machine.TypedSpec().Value.Connected {
		ctrl.runner.StartTask(ctx, logger, machine.Metadata().ID(), spec, ctrl.notifyCh)
	}

	return safe.WriterModify(ctx, r, omni.NewMachineStatus(machine.Metadata().ID()), func(m *omni.MachineStatus) error {
		spec := m.TypedSpec().Value

		helpers.CopyLabels(machine, m, omni.LabelMachineRequest, omni.LabelMachineRequestSet, omni.LabelNoManualAllocation, omni.LabelIsManagedByStaticInfraProvider)

		infraProvider, ok := machine.Metadata().Labels().Get(omni.LabelInfraProviderID)
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

		if errLabels := ctrl.reconcileLabels(m, inputs.infraMachineStatus); errLabels != nil {
			return errLabels
		}

		if err = ctrl.setClusterRelation(inputs, m); err != nil {
			return err
		}

		machinectrl.UpdateConnectionStatus(m, machine, inputs.machineStatusSnapshot, inputs.infraMachineStatus)
		machinectrl.UpdatePowerState(m, machine, inputs.machineStatusSnapshot, inputs.infraMachineStatus)

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

func (ctrl *MachineStatusController) reconcileTearingDown(ctx context.Context, r controller.QRuntime, logger *zap.Logger, machine *omni.Machine) error {
	ctrl.runner.StopTask(logger, machine.Metadata().ID())

	md := omni.NewMachineStatus(machine.Metadata().ID()).Metadata()

	ready, err := helpers.TeardownAndDestroy(ctx, r, md)
	if err != nil {
		return err
	}

	if !ready {
		return nil
	}

	return r.RemoveFinalizer(ctx, machine.Metadata(), ctrl.Name())
}

// populateSchematicRaw fills schematicInfo.Raw for the "meta extension present but no ExtraInfo" case (older Talos)
// by either reusing the raw we already have stored on the machine's MachineStatus or fetching from the image factory.
//
// Once Raw is recovered, KernelArgs is hydrated from it. Otherwise downstream writes empty kernel args into
// MachineStatus and PatchSchematic ends up stripping the real kernel args.
//
// If the factory returns 404, the schematic is not known to it (the machine likely came from a different factory).
// In that case we leave Raw empty and downstream controllers skip schematic management until raw becomes available.
// Any other error (network, auth, marshal failure) is propagated so the reconcile retries.
//
// Raw and FullID are always written together, never out of sync.
func (ctrl *MachineStatusController) populateSchematicRaw(ctx context.Context, info *machinetask.SchematicInfo, existing *omni.MachineStatus, logger *zap.Logger) error {
	if info == nil || info.FullID == "" || info.Raw != "" || info.InAgentMode || info.Invalid {
		return nil
	}

	var raw string

	if existing.TypedSpec().Value.GetSchematic().GetFullId() == info.FullID && existing.TypedSpec().Value.GetSchematic().GetRaw() != "" {
		raw = existing.TypedSpec().Value.Schematic.Raw
	} else {
		logger = logger.With(zap.String("id", existing.Metadata().ID()), zap.String("schematic_id", info.FullID))

		logger.Info("machine does have a schematic ID but no raw schematic, get it from image factory")

		factoryCtx, cancel := context.WithTimeout(ctx, time.Second*30)
		sch, err := ctrl.ImageFactoryClient.SchematicGet(factoryCtx, info.FullID)

		cancel()

		if err != nil {
			if factoryclient.IsHTTPErrorCode(err, http.StatusNotFound) {
				logger.Warn("raw schematic not found in image factory", zap.String("host", ctrl.ImageFactoryClient.Host()))

				return nil
			}

			return fmt.Errorf("failed to fetch schematic %q from image factory: %w", info.FullID, err)
		}

		data, err := sch.Marshal()
		if err != nil {
			return fmt.Errorf("failed to marshal schematic %q fetched from image factory: %w", info.FullID, err)
		}

		raw = string(data)
	}

	parsed, err := schematic.Unmarshal([]byte(raw))
	if err != nil {
		return fmt.Errorf("failed to parse recovered schematic %q: %w", info.FullID, err)
	}

	info.Raw = raw
	info.KernelArgs = parsed.Customization.ExtraKernelArgs

	return nil
}

//nolint:gocyclo,cyclop,gocognit
func (ctrl *MachineStatusController) handleNotification(ctx context.Context, r controller.QRuntime, event machinetask.Info, logger *zap.Logger) error {
	if err := safe.WriterModify(ctx, r, omni.NewMachineStatus(event.MachineID), func(machineStatus *omni.MachineStatus) error {
		if err := ctrl.populateSchematicRaw(ctx, event.Schematic, machineStatus, logger); err != nil {
			return err
		}

		spec := machineStatus.TypedSpec().Value

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

		if event.KernelCmdline != "" {
			spec.KernelCmdline = event.KernelCmdline
		}

		if event.ImageLabels != nil {
			spec.ImageLabels = event.ImageLabels

			machineStatus.Metadata().Labels().Do(func(temp kvutils.TempKV) {
				for key, value := range spec.ImageLabels {
					temp.Set(key, value)
				}
			})
		}

		if event.NoAccess {
			machineStatus.Metadata().Labels().Set(omni.MachineStatusLabelInvalidState, "")
		} else {
			machineStatus.Metadata().Labels().Delete(omni.MachineStatusLabelInvalidState)
		}

		if event.SecurityState != nil {
			spec.SecurityState = event.SecurityState
		}

		if event.Schematic != nil {
			if err := ctrl.handleEventSchematic(ctx, machineStatus, event); err != nil {
				return fmt.Errorf("failed to handle schematic event: %w", err)
			}
		}

		spec.Maintenance = event.MaintenanceMode

		if err := ctrl.reconcileLabels(machineStatus, event.InfraMachineStatus); err != nil {
			return fmt.Errorf("failed to reconcile labels: %w", err)
		}

		// Intentionally do this separately from the usual labels reconciliation, as these
		// are based only on PlatformMetadata tags.
		if machineStatus.TypedSpec().Value.PlatformMetadata != nil {
			if err := ctrl.reconcilePlatformTagLabels(ctx, r, machineStatus); err != nil {
				return err
			}
		}

		return nil
	}); err != nil && !state.IsPhaseConflictError(err) {
		return fmt.Errorf("error modifying resource: %w", err)
	}

	// Backfill the factory if the schematic the machine booted with happens to be missing
	// there (e.g. after a factory swap). Routed through PatchSchematic so the upload
	// preserves all fields the user/factory set on the original schematic (owner,
	// secureboot, bootloader, meta, overlay incl. options, future fields).
	if event.Schematic != nil && event.Schematic.FullID != "" && event.Schematic.Raw != "" {
		machineSchematic, err := imagefactory.PatchSchematic(
			event.Schematic.Raw,
			event.Schematic.Extensions,
			event.Schematic.KernelArgs,
		)
		if err != nil {
			return fmt.Errorf("error patching schematic for re-upload: %w", err)
		}

		factoryCtx, cancel := context.WithTimeout(ctx, time.Second*30)
		_, _, err = ctrl.ImageFactoryClient.EnsureSchematic(factoryCtx, machineSchematic)

		cancel()

		if err != nil {
			return fmt.Errorf("error ensuring schematic: %w", err)
		}
	}

	return nil
}

// reconcilePlatformTagLabels bootstraps a MachineLabels resource from PlatformMetadata tags on the first join
// and stamps the PlatformTagLabelsInitialized annotation on both machineLabels and machineStatus when done.
// Subsequent calls are short-circuited by the annotation on machineLabels.
// We only populate these labels on Omni join; from there on the user can freely modify them.
func (ctrl *MachineStatusController) reconcilePlatformTagLabels(ctx context.Context, r controller.QRuntime, machineStatus *omni.MachineStatus) error {
	if err := safe.WriterModify(ctx, r, omni.NewMachineLabels(machineStatus.Metadata().ID()), func(machineLabels *omni.MachineLabels) error {
		// Skip if annotation is set on either resource, but ensure eventual consistency, in case any of the writes failed before.
		if _, isAnnotated := machineStatus.Metadata().Annotations().Get(omni.PlatformTagLabelsInitialized); isAnnotated {
			machineLabels.Metadata().Annotations().Set(omni.PlatformTagLabelsInitialized, "")

			return nil
		}

		if _, isAnnotated := machineLabels.Metadata().Annotations().Get(omni.PlatformTagLabelsInitialized); isAnnotated {
			machineStatus.Metadata().Annotations().Set(omni.PlatformTagLabelsInitialized, "")

			return nil
		}

		for k, v := range machineStatus.TypedSpec().Value.PlatformMetadata.GetTags() {
			// Do not overwrite pre-existing user labels.
			if _, ok := machineLabels.Metadata().Labels().Get(k); !ok {
				machineLabels.Metadata().Labels().Set(k, v)
			}
		}

		machineLabels.Metadata().Annotations().Set(omni.PlatformTagLabelsInitialized, "")
		machineStatus.Metadata().Annotations().Set(omni.PlatformTagLabelsInitialized, "")

		return nil
	}, controller.WithModifyNoOwner()); err != nil {
		return err
	}

	return nil
}

func (ctrl *MachineStatusController) handleEventSchematic(ctx context.Context, ms *omni.MachineStatus, event machinetask.Info) error {
	spec := ms.TypedSpec().Value

	// If the machine is in agent mode, reset the schematic information as it were never observed.
	// At that stage, anything on it (extensions, kernel args, etc.) is irrelevant and even wrong for Omni to process.
	if event.Schematic.InAgentMode {
		spec.Schematic = &specs.MachineStatusSpec_Schematic{
			InAgentMode: true,
		}

		return nil
	}

	if spec.Schematic == nil {
		spec.Schematic = &specs.MachineStatusSpec_Schematic{}
	}

	spec.Schematic.Raw = event.Schematic.Raw
	spec.Schematic.FullId = event.Schematic.FullID
	spec.Schematic.Extensions = event.Schematic.Extensions
	spec.Schematic.Invalid = event.Schematic.Invalid
	spec.Schematic.KernelArgs = event.Schematic.KernelArgs
	spec.Schematic.InAgentMode = event.Schematic.InAgentMode

	if spec.Schematic.InitialSchematic == "" {
		spec.Schematic.InitialSchematic = spec.Schematic.FullId
	}

	// We populate the initial state on a best-effort basis: we might have missed the moment to capture it, but it is acceptable.
	if spec.Schematic.InitialState == nil {
		spec.Schematic.InitialState = &specs.MachineStatusSpec_Schematic_InitialState{
			Extensions: event.Schematic.Extensions,
		}
	}

	// If the schematic is invalid, reset the initial schematic information.
	if spec.Schematic.Invalid {
		spec.Schematic.InitialSchematic = ""
		spec.Schematic.InitialState = &specs.MachineStatusSpec_Schematic_InitialState{} // reset to be empty but leave it initialized
	}

	_, kernelArgsInitialized := ms.Metadata().Annotations().Get(omni.KernelArgsInitialized)
	if !kernelArgsInitialized {
		if err := ctrl.kernelArgsInitializer.Init(ctx, ms.Metadata().ID(), event.Schematic.KernelArgs); err != nil {
			return fmt.Errorf("failed to initialize extra kernel args: %w", err)
		}

		ms.Metadata().Annotations().Set(omni.KernelArgsInitialized, "")
	}

	return nil
}

// reconcileLabels is a wrapper around omni.ReconcileMachineStatusLabels, but it overrides the "installed" label
// based on a matching infra.MachineStatus for that machine.
//
// This is because if there is a matching infra.MachineStatus, it means that the machine is managed by a static infrastructure provider (e.g., the bare-metal provider),
// and it might be powered off by the provider. In that case, the block disks information on the MachineStatus might be not up to date,
// and the information on infra.MachineStatus will be more reliable.
func (ctrl *MachineStatusController) reconcileLabels(machineStatus *omni.MachineStatus, infraMachineStatus *infra.MachineStatus) error {
	omni.ReconcileMachineStatusLabels(machineStatus)

	if infraMachineStatus != nil {
		if infraMachineStatus.TypedSpec().Value.Installed {
			machineStatus.Metadata().Labels().Set(omni.MachineStatusLabelInstalled, "")
		} else {
			machineStatus.Metadata().Labels().Delete(omni.MachineStatusLabelInstalled)
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

func (ctrl *MachineStatusController) handleInputs(ctx context.Context, r controller.Reader, machine *omni.Machine) (inputs, error) {
	var (
		in  inputs
		err error
	)

	in.machineStatusSnapshot, err = safe.ReaderGetByID[*omni.MachineStatusSnapshot](ctx, r, machine.Metadata().ID())
	if err != nil && !state.IsNotFoundError(err) {
		return in, err
	}

	in.machineLabels, err = safe.ReaderGetByID[*omni.MachineLabels](ctx, r, machine.Metadata().ID())
	if err != nil && !state.IsNotFoundError(err) {
		return in, err
	}

	in.clusterMachineStatus, err = safe.ReaderGetByID[*omni.ClusterMachineStatus](ctx, r, machine.Metadata().ID())
	if err != nil && !state.IsNotFoundError(err) {
		return in, err
	}

	in.machineSetNode, err = safe.ReaderGetByID[*omni.MachineSetNode](ctx, r, machine.Metadata().ID())
	if err != nil && !state.IsNotFoundError(err) {
		return in, err
	}

	in.infraMachineStatus, err = safe.ReaderGetByID[*infra.MachineStatus](ctx, r, machine.Metadata().ID())
	if err != nil && !state.IsNotFoundError(err) {
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

func (ctrl *MachineStatusController) mergeLabels(machineStatus *omni.MachineStatus, machineLabels *omni.MachineLabels) map[string]string {
	labels := map[string]string{}

	if machineStatus.TypedSpec().Value.ImageLabels != nil {
		maps.Copy(labels, machineStatus.TypedSpec().Value.ImageLabels)
	}

	if machineLabels != nil {
		maps.Copy(labels, machineLabels.Metadata().Labels().Raw())
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
		machineStatus.Metadata().Labels().Delete(omni.LabelMachineSet)
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

	machineSet, machineSetOk := labels.Get(omni.LabelMachineSet)
	if !machineSetOk {
		return fmt.Errorf("malformed ClusterMachine resource: no %q label, machine set ownership unknown", omni.LabelMachineSet)
	}

	machineStatus.Metadata().Labels().Set(omni.LabelCluster, cluster)
	machineStatus.Metadata().Labels().Set(omni.LabelMachineSet, machineSet)

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
