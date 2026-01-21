// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"fmt"
	"slices"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/google/uuid"
	"github.com/siderolabs/gen/xiter"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
)

// InfraMachineControllerName is the name of the controller.
const InfraMachineControllerName = "InfraMachineController"

// InfraMachineController manages InfraMachine resource lifecycle.
//
// InfraMachineController transforms an Omni Machine managed by a static infra provider to an infra.Machine, applying the user overrides in omni.InfraMachineConfig resource if present.
type InfraMachineController struct {
	installEventCh <-chan resource.ID
	generic.NamedController
}

// NewInfraMachineController creates a new InfraMachineController.
func NewInfraMachineController(installEventCh <-chan resource.ID) *InfraMachineController {
	return &InfraMachineController{
		installEventCh: installEventCh,
		NamedController: generic.NamedController{
			ControllerName: InfraMachineControllerName,
		},
	}
}

// Settings implements the controller.QController interface.
//
//nolint:dupl
func (ctrl *InfraMachineController) Settings() controller.QSettings {
	return controller.QSettings{
		Inputs: []controller.Input{
			{
				Namespace: resources.DefaultNamespace,
				Type:      siderolink.LinkType,
				Kind:      controller.InputQPrimary,
			},
			{
				Namespace: resources.DefaultNamespace,
				Type:      siderolink.NodeUniqueTokenType,
				Kind:      controller.InputQMapped,
			},
			{
				Namespace: resources.InfraProviderNamespace,
				Type:      infra.InfraMachineType,
				Kind:      controller.InputQMappedDestroyReady,
			},
			{
				Namespace: resources.DefaultNamespace,
				Type:      omni.InfraMachineConfigType,
				Kind:      controller.InputQMapped,
			},
			{
				Namespace: resources.DefaultNamespace,
				Type:      omni.SchematicConfigurationType,
				Kind:      controller.InputQMapped,
			},
			{
				Namespace: resources.DefaultNamespace,
				Type:      omni.MachineExtensionsType,
				Kind:      controller.InputQMapped,
			},
			{
				Namespace: resources.DefaultNamespace,
				Type:      omni.ClusterMachineType,
				Kind:      controller.InputQMapped,
			},
			{
				Namespace: resources.DefaultNamespace,
				Type:      omni.MachineStatusType,
				Kind:      controller.InputQMapped,
			},
			{
				Namespace: resources.InfraProviderNamespace,
				Type:      infra.InfraProviderStatusType,
				Kind:      controller.InputQMapped,
			},
			{
				Namespace: resources.InfraProviderNamespace,
				Type:      infra.ProviderType,
				Kind:      controller.InputQMapped,
			},
		},
		Outputs: []controller.Output{
			{
				Kind: controller.OutputExclusive,
				Type: infra.InfraMachineType,
			},
		},
		RunHook: func(ctx context.Context, _ *zap.Logger, r controller.QRuntime) error {
			for {
				select {
				case <-ctx.Done():
					return nil
				case machineID := <-ctrl.installEventCh:
					if err := ctrl.handleInstallEvent(ctx, r, machineID); err != nil {
						return err
					}
				}
			}
		},
	}
}

// Reconcile implements the controller.QController interface.
func (ctrl *InfraMachineController) Reconcile(ctx context.Context, _ *zap.Logger, r controller.QRuntime, ptr resource.Pointer) error {
	link, err := safe.ReaderGet[*siderolink.Link](ctx, r, ptr)
	if err != nil {
		if !state.IsNotFoundError(err) {
			return err
		}

		// the link is not found, so we prepare a fake link resource to trigger teardown logic
		link = siderolink.NewLink(ptr.ID(), nil)
		link.Metadata().SetPhase(resource.PhaseTearingDown)
	}

	if link.Metadata().Phase() == resource.PhaseTearingDown {
		return ctrl.reconcileTearingDown(ctx, r, link)
	}

	return ctrl.reconcileRunning(ctx, r, link)
}

func (ctrl *InfraMachineController) reconcileTearingDown(ctx context.Context, r controller.QRuntime, link *siderolink.Link) error {
	md := infra.NewMachine(link.Metadata().ID()).Metadata()

	clusterMachine, err := safe.ReaderGetByID[*omni.ClusterMachine](ctx, r, link.Metadata().ID())
	if err != nil && !state.IsNotFoundError(err) {
		return err
	}

	if clusterMachine != nil && clusterMachine.Metadata().Phase() == resource.PhaseTearingDown {
		if clusterMachine.Metadata().Finalizers().Has(ClusterMachineConfigControllerName) {
			return nil // the cluster machine is not reset yet
		}

		if err = r.RemoveFinalizer(ctx, clusterMachine.Metadata(), ctrl.Name()); err != nil {
			return err
		}
	}

	if err = ctrl.handleInfraProviderDeletion(ctx, r, link); err != nil {
		return err
	}

	ready, err := helpers.TeardownAndDestroy(ctx, r, md)
	if err != nil {
		return err
	}

	if !ready {
		return nil
	}

	return r.RemoveFinalizer(ctx, link.Metadata(), ctrl.Name())
}

func (ctrl *InfraMachineController) handleInfraProviderDeletion(ctx context.Context, r controller.QRuntime, link *siderolink.Link) error {
	infraProviderID, ok := link.Metadata().Labels().Get(omni.LabelInfraProviderID)
	if !ok {
		return nil
	}

	infraProvider, err := safe.ReaderGetByID[*infra.Provider](ctx, r, infraProviderID)
	if err != nil && !state.IsNotFoundError(err) {
		return err
	}

	// Remove all finalizers from the infra machine if the infra provider is being deleted or not found.
	if infraProvider != nil && infraProvider.Metadata().Phase() == resource.PhaseRunning {
		return nil
	}

	machine, err := safe.ReaderGetByID[*infra.Machine](ctx, r, link.Metadata().ID())
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil
		}

		return err
	}

	return safe.WriterModify[*infra.Machine](ctx, r, machine, func(r *infra.Machine) error {
		for _, finalizer := range *machine.Metadata().Finalizers() {
			r.Metadata().Finalizers().Remove(finalizer)
		}

		return nil
	}, controller.WithExpectedPhaseAny())
}

func (ctrl *InfraMachineController) reconcileRunning(ctx context.Context, r controller.QRuntime, link *siderolink.Link) error {
	config, err := safe.ReaderGetByID[*omni.InfraMachineConfig](ctx, r, link.Metadata().ID())
	if err != nil && !state.IsNotFoundError(err) {
		return err
	}

	machineExts, err := safe.ReaderGetByID[*omni.MachineExtensions](ctx, r, link.Metadata().ID())
	if err != nil && !state.IsNotFoundError(err) {
		return err
	}

	machineStatus, err := safe.ReaderGetByID[*omni.MachineStatus](ctx, r, link.Metadata().ID())
	if err != nil && !state.IsNotFoundError(err) {
		return err
	}

	nodeUniqueToken, err := safe.ReaderGetByID[*siderolink.NodeUniqueToken](ctx, r, link.Metadata().ID())
	if err != nil && !state.IsNotFoundError(err) {
		return err
	}

	providerID, ok := link.Metadata().Labels().Get(omni.LabelInfraProviderID)
	if !ok {
		return nil // the link is not created by an infra provider
	}

	providerStatus, err := safe.ReaderGetByID[*infra.ProviderStatus](ctx, r, providerID)
	if err != nil {
		return err
	}

	if _, isStaticProvider := providerStatus.Metadata().Labels().Get(omni.LabelIsStaticInfraProvider); !isStaticProvider {
		return nil // the link is not created by a static infra provider
	}

	machineInfoCollected := machineStatus != nil && machineStatus.TypedSpec().Value.SecurityState != nil

	helper := &infraMachineControllerHelper{
		config:               config,
		machineExts:          machineExts,
		link:                 link,
		nodeUniqueToken:      nodeUniqueToken,
		runtime:              r,
		machineInfoCollected: machineInfoCollected,
		providerID:           providerID,
		controllerName:       ctrl.Name(),
	}

	return safe.WriterModify[*infra.Machine](ctx, r, infra.NewMachine(link.Metadata().ID()), func(res *infra.Machine) error {
		return helper.modify(ctx, res)
	})
}

type infraMachineControllerHelper struct {
	runtime              controller.QRuntime
	config               *omni.InfraMachineConfig
	machineExts          *omni.MachineExtensions
	link                 *siderolink.Link
	nodeUniqueToken      *siderolink.NodeUniqueToken
	providerID           string
	controllerName       string
	machineInfoCollected bool
}

func (helper *infraMachineControllerHelper) modify(ctx context.Context, infraMachine *infra.Machine) error {
	if err := helper.applyInfraMachineConfig(infraMachine, helper.config, helper.machineInfoCollected); err != nil {
		return err
	}

	if err := helper.runtime.AddFinalizer(ctx, helper.link.Metadata(), helper.controllerName); err != nil {
		return err
	}

	infraMachine.Metadata().Labels().Set(omni.LabelInfraProviderID, helper.providerID)

	if helper.nodeUniqueToken != nil {
		infraMachine.TypedSpec().Value.NodeUniqueToken = helper.nodeUniqueToken.TypedSpec().Value.Token
	}

	clusterMachine, err := safe.ReaderGetByID[*omni.ClusterMachine](ctx, helper.runtime, helper.link.Metadata().ID())
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil
		}

		return err
	}

	helpers.CopyLabels(clusterMachine, infraMachine, omni.LabelCluster, omni.LabelMachineSet, omni.LabelControlPlaneRole, omni.LabelWorkerRole)

	if clusterMachine.Metadata().Phase() == resource.PhaseTearingDown {
		if clusterMachine.Metadata().Finalizers().Has(ClusterMachineConfigControllerName) {
			return nil // the cluster machine is not reset yet
		}

		// the machine is deallocated, clear the cluster information and mark it for wipe by assigning it a new wipe ID
		if infraMachine.TypedSpec().Value.ClusterTalosVersion != "" {
			infraMachine.TypedSpec().Value.WipeId = uuid.NewString()
		}

		infraMachine.TypedSpec().Value.ClusterTalosVersion = ""
		infraMachine.TypedSpec().Value.Extensions = nil

		infraMachine.Metadata().Labels().Delete(omni.LabelCluster)
		infraMachine.Metadata().Labels().Delete(omni.LabelMachineSet)
		infraMachine.Metadata().Labels().Delete(omni.LabelControlPlaneRole)
		infraMachine.Metadata().Labels().Delete(omni.LabelWorkerRole)

		if err = helper.runtime.RemoveFinalizer(ctx, clusterMachine.Metadata(), helper.controllerName); err != nil {
			return err
		}

		return nil
	}

	if err = helper.runtime.AddFinalizer(ctx, clusterMachine.Metadata(), helper.controllerName); err != nil {
		return err
	}

	schematicConfig, err := safe.ReaderGetByID[*omni.SchematicConfiguration](ctx, helper.runtime, helper.link.Metadata().ID())
	if err != nil {
		if state.IsNotFoundError(err) {
			// the schema configuration is not created yet, skip the cluster information collection
			return nil
		}

		return err
	}

	var extensions []string

	if helper.machineExts != nil {
		extensions = helper.machineExts.TypedSpec().Value.Extensions
	}

	// set the cluster allocation information

	infraMachine.TypedSpec().Value.ClusterTalosVersion = schematicConfig.TypedSpec().Value.TalosVersion
	infraMachine.TypedSpec().Value.Extensions = extensions

	return nil
}

// MapInput implements the controller.QController interface.
func (ctrl *InfraMachineController) MapInput(ctx context.Context, _ *zap.Logger, runtime controller.QRuntime, ptr controller.ReducedResourceMetadata) ([]resource.Pointer, error) {
	switch ptr.Type() {
	case siderolink.LinkType,
		siderolink.NodeUniqueTokenType,
		infra.InfraMachineType,
		omni.InfraMachineConfigType,
		omni.SchematicConfigurationType,
		omni.MachineExtensionsType,
		omni.ClusterMachineType,
		omni.MachineStatusType:
		return []resource.Pointer{siderolink.NewLink(ptr.ID(), nil).Metadata()}, nil
	case infra.InfraProviderStatusType, infra.ProviderType:
		linkList, err := safe.ReaderListAll[*siderolink.Link](ctx, runtime, state.WithLabelQuery(resource.LabelEqual(omni.LabelInfraProviderID, ptr.ID())))
		if err != nil {
			return nil, err
		}

		ptrSeq := xiter.Map(func(in *siderolink.Link) resource.Pointer {
			return in.Metadata()
		}, linkList.All())

		return slices.Collect(ptrSeq), nil
	}

	return nil, fmt.Errorf("unexpected resource type %q", ptr.Type())
}

func (ctrl *InfraMachineController) handleInstallEvent(ctx context.Context, r controller.QRuntime, machineID resource.ID) error {
	if _, err := safe.ReaderGetByID[*infra.Machine](ctx, r, machineID); err != nil {
		if state.IsNotFoundError(err) {
			return nil // if there is no infra machine, there is nothing to do
		}

		return err
	}

	return safe.WriterModify(ctx, r, infra.NewMachine(machineID), func(machine *infra.Machine) error {
		if machine.Metadata().Phase() == resource.PhaseTearingDown {
			return nil
		}

		machine.TypedSpec().Value.InstallEventId++

		return nil
	})
}

// applyInfraMachineConfig applies the user-managed configuration from the omni.InfraMachineConfig resource into the infra.Machine.
func (helper *infraMachineControllerHelper) applyInfraMachineConfig(infraMachine *infra.Machine, config *omni.InfraMachineConfig, machineInfoCollected bool) error {
	const defaultPreferredPowerState = specs.InfraMachineSpec_POWER_STATE_OFF // todo: introduce a resource to configure this globally or per-provider level

	// reset the user-override fields except the "Accepted" field
	infraMachine.TypedSpec().Value.PreferredPowerState = defaultPreferredPowerState
	infraMachine.TypedSpec().Value.ExtraKernelArgs = ""

	pendingAccept := config == nil

	if config != nil { // apply user configuration: acceptance, preferred power state, extra kernel args, requested reboot id
		infraMachine.TypedSpec().Value.RequestedRebootId = config.TypedSpec().Value.RequestedRebootId
		infraMachine.TypedSpec().Value.AcceptanceStatus = config.TypedSpec().Value.AcceptanceStatus
		infraMachine.TypedSpec().Value.Cordoned = config.TypedSpec().Value.Cordoned

		pendingAccept = infraMachine.TypedSpec().Value.AcceptanceStatus == specs.InfraMachineConfigSpec_PENDING

		switch config.TypedSpec().Value.PowerState {
		case specs.InfraMachineConfigSpec_POWER_STATE_OFF:
			infraMachine.TypedSpec().Value.PreferredPowerState = specs.InfraMachineSpec_POWER_STATE_OFF
		case specs.InfraMachineConfigSpec_POWER_STATE_ON:
			infraMachine.TypedSpec().Value.PreferredPowerState = specs.InfraMachineSpec_POWER_STATE_ON
		case specs.InfraMachineConfigSpec_POWER_STATE_DEFAULT:
			infraMachine.TypedSpec().Value.PreferredPowerState = defaultPreferredPowerState
		default:
			return fmt.Errorf("unknown power state: %v", config.TypedSpec().Value.PowerState.String())
		}

		infraMachine.TypedSpec().Value.ExtraKernelArgs = config.TypedSpec().Value.ExtraKernelArgs
	}

	if pendingAccept {
		infraMachine.Metadata().Labels().Set(omni.LabelMachinePendingAccept, "")
	} else {
		infraMachine.Metadata().Labels().Delete(omni.LabelMachinePendingAccept)
	}

	if !machineInfoCollected { // we need the machine to stay powered on even if it is accepted, until Omni collects the machine information
		infraMachine.TypedSpec().Value.PreferredPowerState = specs.InfraMachineSpec_POWER_STATE_ON
	}

	return nil
}
