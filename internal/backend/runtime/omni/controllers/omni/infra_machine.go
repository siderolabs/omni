// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"fmt"
	"slices"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/google/uuid"
	"github.com/siderolabs/gen/xerrors"
	"github.com/siderolabs/gen/xiter"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
)

// InfraMachineControllerName is the name of the controller.
const InfraMachineControllerName = "InfraMachineController"

// InfraMachineController manages InfraMachine resource lifecycle.
//
// InfraMachineController transforms an Omni Machine managed by a static infra provider to an infra.Machine, applying the user overrides in omni.InfraMachineConfig resource if present.
type InfraMachineController = qtransform.QController[*siderolink.Link, *infra.Machine]

// NewInfraMachineController initializes InfraMachineController.
func NewInfraMachineController() *InfraMachineController {
	helper := &infraMachineControllerHelper{}

	return qtransform.NewQController(
		qtransform.Settings[*siderolink.Link, *infra.Machine]{
			Name: InfraMachineControllerName,
			MapMetadataFunc: func(link *siderolink.Link) *infra.Machine {
				return infra.NewMachine(link.Metadata().ID())
			},
			UnmapMetadataFunc: func(infraMachine *infra.Machine) *siderolink.Link {
				return siderolink.NewLink(resources.DefaultNamespace, infraMachine.Metadata().ID(), nil)
			},
			TransformExtraOutputFunc:        helper.transformExtraOutput,
			FinalizerRemovalExtraOutputFunc: helper.finalizerRemovalExtraOutput,
		},
		qtransform.WithExtraMappedInput(
			qtransform.MapperSameID[*omni.InfraMachineConfig, *siderolink.Link](),
		),
		qtransform.WithExtraMappedInput(
			qtransform.MapperSameID[*omni.SchematicConfiguration, *siderolink.Link](),
		),
		qtransform.WithExtraMappedInput(
			qtransform.MapperSameID[*omni.MachineExtensions, *siderolink.Link](),
		),
		qtransform.WithExtraMappedInput(
			qtransform.MapperSameID[*omni.ClusterMachine, *siderolink.Link](),
		),
		qtransform.WithExtraMappedInput(
			func(ctx context.Context, _ *zap.Logger, runtime controller.QRuntime, res *infra.ProviderStatus) ([]resource.Pointer, error) {
				linkList, err := safe.ReaderListAll[*siderolink.Link](ctx, runtime, state.WithLabelQuery(resource.LabelEqual(omni.LabelInfraProviderID, res.Metadata().ID())))
				if err != nil {
					return nil, err
				}

				ptrSeq := xiter.Map(func(in *siderolink.Link) resource.Pointer {
					return in.Metadata()
				}, linkList.All())

				return slices.Collect(ptrSeq), nil
			},
		),
	)
}

type infraMachineControllerHelper struct{}

func (h *infraMachineControllerHelper) transformExtraOutput(ctx context.Context, r controller.ReaderWriter, _ *zap.Logger, link *siderolink.Link, infraMachine *infra.Machine) error {
	providerID, ok := link.Metadata().Annotations().Get(omni.LabelInfraProviderID)
	if !ok {
		return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("the link is not created by an infra provider")
	}

	providerStatus, err := safe.ReaderGetByID[*infra.ProviderStatus](ctx, r, providerID)
	if err != nil {
		return err
	}

	if _, isStaticProvider := providerStatus.Metadata().Labels().Get(omni.LabelIsStaticInfraProvider); !isStaticProvider {
		return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("the link is not created by a static infra provider")
	}

	if err = h.applyInfraMachineConfig(ctx, r, link, infraMachine); err != nil {
		return err
	}

	infraMachine.Metadata().Labels().Set(omni.LabelInfraProviderID, providerID)

	clusterMachine, err := safe.ReaderGetByID[*omni.ClusterMachine](ctx, r, link.Metadata().ID())
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil
		}

		return err
	}

	if clusterMachine.Metadata().Phase() == resource.PhaseTearingDown {
		if clusterMachine.Metadata().Finalizers().Has(ClusterMachineConfigControllerName) {
			return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("cluster machine is not reset yet")
		}

		// the machine is deallocated, clear the cluster information and mark it for wipe by assigning it a new wipe ID

		infraMachine.TypedSpec().Value.ClusterTalosVersion = ""
		infraMachine.TypedSpec().Value.Extensions = nil
		infraMachine.TypedSpec().Value.WipeId = uuid.NewString()

		if err = r.RemoveFinalizer(ctx, clusterMachine.Metadata(), InfraMachineControllerName); err != nil {
			return err
		}

		return nil
	}

	if err = r.AddFinalizer(ctx, clusterMachine.Metadata(), InfraMachineControllerName); err != nil {
		return err
	}

	talosVersion, extensions, err := h.getClusterInfo(ctx, r, link.Metadata().ID())
	if err != nil {
		return err
	}

	// set the cluster allocation information

	infraMachine.TypedSpec().Value.ClusterTalosVersion = talosVersion
	infraMachine.TypedSpec().Value.Extensions = extensions

	return nil
}

func (h *infraMachineControllerHelper) finalizerRemovalExtraOutput(ctx context.Context, r controller.ReaderWriter, _ *zap.Logger, link *siderolink.Link) error {
	clusterMachine, err := safe.ReaderGetByID[*omni.ClusterMachine](ctx, r, link.Metadata().ID())
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil
		}

		return err
	}

	if err = r.RemoveFinalizer(ctx, clusterMachine.Metadata(), InfraMachineControllerName); err != nil {
		return err
	}

	return nil
}

// applyInfraMachineConfig applies the user-managed configuration from the omni.InfraMachineConfig resource into the infra.Machine.
func (h *infraMachineControllerHelper) applyInfraMachineConfig(ctx context.Context, r controller.Reader, link *siderolink.Link, infraMachine *infra.Machine) error {
	config, err := safe.ReaderGetByID[*omni.InfraMachineConfig](ctx, r, link.Metadata().ID())
	if err != nil && !state.IsNotFoundError(err) {
		return err
	}

	const defaultPreferredPowerState = specs.InfraMachineSpec_POWER_STATE_OFF // todo: introduce a resource to configure this globally or per-provider level

	// reset the user-override fields except the "Accepted" field
	infraMachine.TypedSpec().Value.PreferredPowerState = defaultPreferredPowerState
	infraMachine.TypedSpec().Value.ExtraKernelArgs = ""

	if config != nil { // apply user configuration: acceptance, preferred power state, extra kernel args
		infraMachine.TypedSpec().Value.AcceptanceStatus = config.TypedSpec().Value.AcceptanceStatus

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

	return nil
}

// getClusterInfo returns the Talos version and extensions for the given machine.
//
// At this point, the machine is known to be associated with a cluster.
func (h *infraMachineControllerHelper) getClusterInfo(ctx context.Context, r controller.Reader, id resource.ID) (string, []string, error) {
	schematicConfig, err := safe.ReaderGetByID[*omni.SchematicConfiguration](ctx, r, id)
	if err != nil {
		if state.IsNotFoundError(err) {
			return "", nil, xerrors.NewTaggedf[qtransform.SkipReconcileTag]("schema configuration is not created yet")
		}

		return "", nil, err
	}

	machineExts, err := safe.ReaderGetByID[*omni.MachineExtensions](ctx, r, schematicConfig.Metadata().ID())
	if err != nil {
		if state.IsNotFoundError(err) { // no extensions
			return schematicConfig.TypedSpec().Value.TalosVersion, nil, nil
		}

		return "", nil, err
	}

	return schematicConfig.TypedSpec().Value.TalosVersion, machineExts.TypedSpec().Value.Extensions, nil
}
