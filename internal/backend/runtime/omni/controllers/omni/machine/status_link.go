// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package machine

import (
	"context"
	"fmt"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xerrors"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/cosi/helpers"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	siderolinkmanager "github.com/siderolabs/omni/internal/pkg/siderolink"
)

// NewStatusLinkController creates new StatusLinkController.
func NewStatusLinkController(linkCounterDeltaCh <-chan siderolinkmanager.LinkCounterDeltas) *StatusLinkController {
	ctrl := &StatusLinkController{
		deltaCh: linkCounterDeltaCh,
		NamedController: generic.NamedController{
			ControllerName: "MachineStatusLinkController",
		},
	}

	return ctrl
}

// StatusLinkController joins [omni.MachineStatus] and link counter deltas from SideroLink manager into one virtual resource which will
// be stored in [resources.MetricsNamespace].
type StatusLinkController struct {
	deltaCh <-chan siderolinkmanager.LinkCounterDeltas
	generic.NamedController
}

// Settings implements controller.Controller interface.
func (ctrl *StatusLinkController) Settings() controller.QSettings {
	return controller.QSettings{
		Inputs: []controller.Input{
			{
				Namespace: resources.DefaultNamespace,
				Type:      siderolink.LinkType,
				Kind:      controller.InputQPrimary,
			},
			{
				Namespace: resources.DefaultNamespace,
				Type:      omni.MachineStatusType,
				Kind:      controller.InputQMapped,
			},
			{
				Namespace: resources.MetricsNamespace,
				Type:      omni.MachineStatusLinkType,
				Kind:      controller.InputQMappedDestroyReady,
			},
		},
		Outputs: []controller.Output{
			{
				Type: omni.MachineStatusLinkType,
				Kind: controller.OutputExclusive,
			},
		},
		RunHook: func(ctx context.Context, logger *zap.Logger, r controller.QRuntime) error {
			for {
				select {
				case <-ctx.Done():
					return nil
				case deltas, ok := <-ctrl.deltaCh:
					if !ok {
						return nil
					}

					for id, delta := range deltas {
						logger.Debug("reconcile counters", zap.String("id", id))

						if err := ctrl.reconcileDelta(ctx, r, id, delta); err != nil {
							return fmt.Errorf("error reconciling delta for link %s: %w", id, err)
						}
					}
				}
			}
		},
	}
}

// MapInput implements controller.QController interface.
func (ctrl *StatusLinkController) MapInput(ctx context.Context, logger *zap.Logger, r controller.QRuntime, ptr controller.ReducedResourceMetadata) ([]resource.Pointer, error) {
	switch ptr.Type() {
	case omni.MachineStatusType, omni.MachineStatusLinkType:
		return []resource.Pointer{
			siderolink.NewLink(ptr.ID(), nil).Metadata(),
		}, nil
	}

	return nil, fmt.Errorf("unexpected input type: %s", ptr.Type())
}

// Reconcile implements controller.QController interface.
func (ctrl *StatusLinkController) Reconcile(ctx context.Context, logger *zap.Logger, r controller.QRuntime, ptr resource.Pointer) error {
	link, err := safe.ReaderGetByID[*siderolink.Link](ctx, r, ptr.ID())
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil
		}

		return err
	}

	if link.Metadata().Phase() == resource.PhaseRunning {
		if !link.Metadata().Finalizers().Has(ctrl.Name()) {
			if err = r.AddFinalizer(ctx, link.Metadata(), ctrl.Name()); err != nil {
				return fmt.Errorf("error adding finalizer to link resource: %w", err)
			}
		}

		machineStatusLink := omni.NewMachineStatusLink(link.Metadata().ID())

		if err = ctrl.reconcileRunning(ctx, r, link, machineStatusLink); err != nil {
			if xerrors.TagIs[qtransform.SkipReconcileTag](err) {
				logger.Debug("reconcile skipped", zap.Error(err))

				return nil
			}

			return err
		}

		return safe.WriterModify(ctx, r, machineStatusLink, func(res *omni.MachineStatusLink) error {
			*res.Metadata().Labels() = *machineStatusLink.Metadata().Labels()

			// Preserve the SiderolinkCounter set by reconcileDelta, as it's updated independently.
			siderolinkCounter := res.TypedSpec().Value.SiderolinkCounter

			res.TypedSpec().Value = machineStatusLink.TypedSpec().Value

			res.TypedSpec().Value.SiderolinkCounter = siderolinkCounter

			return nil
		})
	}

	return ctrl.reconcileTearingDown(ctx, r, link)
}

func (ctrl *StatusLinkController) reconcileDelta(ctx context.Context, r controller.QRuntime, id string, delta siderolinkmanager.LinkCounterDelta) error {
	_, err := safe.ReaderGetByID[*siderolink.Link](ctx, r, id)
	if err != nil {
		if state.IsNotFoundError(err) {
			// Link resource not found, nothing to update.
			return nil
		}

		return err
	}

	err = safe.WriterModify(ctx, r, omni.NewMachineStatusLink(id), func(res *omni.MachineStatusLink) error {
		if res.TypedSpec().Value.SiderolinkCounter == nil {
			res.TypedSpec().Value.SiderolinkCounter = &specs.SiderolinkCounterSpec{}
		}

		res.TypedSpec().Value.SiderolinkCounter.BytesReceived += delta.BytesReceived
		res.TypedSpec().Value.SiderolinkCounter.BytesSent += delta.BytesSent
		res.TypedSpec().Value.SiderolinkCounter.LastAlive = pickTime(delta.LastAlive, res.TypedSpec().Value.SiderolinkCounter.LastAlive)

		return nil
	})
	if err != nil && !state.IsPhaseConflictError(err) {
		return err
	}

	return nil
}

func (ctrl *StatusLinkController) reconcileRunning(ctx context.Context, r controller.Reader, link *siderolink.Link, machineStatusLink *omni.MachineStatusLink) error {
	machineStatus, err := safe.ReaderGetByID[*omni.MachineStatus](ctx, r, link.Metadata().ID())
	if err != nil {
		if state.IsNotFoundError(err) {
			return xerrors.NewTagged[qtransform.SkipReconcileTag](err)
		}

		return err
	}

	if machineStatus.Metadata().Phase() == resource.PhaseTearingDown {
		return xerrors.NewTagged[qtransform.SkipReconcileTag](fmt.Errorf("machine status is tearing down"))
	}

	// Just copy labels and metadata from MachineStatus resource.
	// This should be safe since Labels are copy-on-write.
	*machineStatusLink.Metadata().Labels() = *machineStatus.Metadata().Labels()

	machineStatusLink.TypedSpec().Value.MessageStatus = machineStatus.TypedSpec().Value
	machineStatusLink.TypedSpec().Value.MachineCreatedAt = machineStatus.Metadata().Created().Unix()

	return nil
}

func (ctrl *StatusLinkController) reconcileTearingDown(ctx context.Context, r controller.QRuntime, link *siderolink.Link) error {
	if !link.Metadata().Finalizers().Has(ctrl.Name()) {
		return nil
	}

	if err := safe.WriterModify(ctx, r, omni.NewMachineStatusLink(link.Metadata().ID()), func(res *omni.MachineStatusLink) error {
		// Just copy labels and metadata from Link resource to preserve the information about the tearing down link.
		// This should be safe since Labels are copy-on-write.
		*res.Metadata().Labels() = *link.Metadata().Labels()

		res.TypedSpec().Value.TearingDown = true

		return nil
	}, controller.WithExpectedPhaseAny()); err != nil {
		return fmt.Errorf("error modifying MachineStatusLink resource: %w", err)
	}

	if len(*link.Metadata().Finalizers()) > 1 {
		return nil
	}

	ready, err := helpers.TeardownAndDestroy(ctx, r, omni.NewMachineStatusLink(link.Metadata().ID()).Metadata())
	if err != nil {
		return fmt.Errorf("error tearing down MachineStatusLink resource: %w", err)
	}

	if !ready {
		return nil
	}

	return r.RemoveFinalizer(ctx, link.Metadata(), ctrl.Name())
}

func pickTime(newAlive time.Time, lastAlive *timestamppb.Timestamp) *timestamppb.Timestamp {
	if newAlive.IsZero() {
		return lastAlive
	}

	if lastAlive == nil || !lastAlive.IsValid() {
		return timestamppb.New(newAlive)
	}

	if newAlive.After(lastAlive.AsTime()) {
		return timestamppb.New(newAlive)
	}

	return lastAlive
}
