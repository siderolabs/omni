// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"fmt"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/siderolabs/gen/xiter"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/cosi/helpers"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	siderolinkmanager "github.com/siderolabs/omni/internal/pkg/siderolink"
)

// NewMachineStatusLinkController creates new MachineStatusLinkController.
func NewMachineStatusLinkController(linkCounterDeltaCh <-chan siderolinkmanager.LinkCounterDeltas) *MachineStatusLinkController {
	return &MachineStatusLinkController{
		deltaCh: linkCounterDeltaCh,
	}
}

// MachineStatusLinkController joins [omni.MachineStatus] and link counter deltas from SideroLink manager into one virtual resource which will
// be stored in [resources.EphemeralNamespace].
type MachineStatusLinkController struct {
	deltaCh <-chan siderolinkmanager.LinkCounterDeltas
}

// Name implements controller.Controller interface.
func (ctrl *MachineStatusLinkController) Name() string {
	return "MachineStatusLinkController"
}

// Inputs implements controller.Controller interface.
func (ctrl *MachineStatusLinkController) Inputs() []controller.Input {
	return []controller.Input{
		safe.Input[*omni.MachineStatus](controller.InputWeak),
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *MachineStatusLinkController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: omni.MachineStatusLinkType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *MachineStatusLinkController) Run(ctx context.Context, rt controller.Runtime, _ *zap.Logger) error {
	for {
		var deltas siderolinkmanager.LinkCounterDeltas

		select {
		case <-ctx.Done():
			return nil
		case <-rt.EventCh():
		case deltas = <-ctrl.deltaCh:
		}

		rt.StartTrackingOutputs()

		msList, err := safe.ReaderListAll[*omni.MachineStatus](ctx, rt)
		if err != nil {
			return fmt.Errorf("error listing MachineStatus resources: %w", err)
		}

		for ms := range msList.All() {
			if ms.Metadata().Phase() == resource.PhaseTearingDown {
				continue
			}

			emptyResource := omni.NewMachineStatusLink(resources.MetricsNamespace, ms.Metadata().ID())

			err = safe.WriterModify(ctx, rt, emptyResource, func(msl *omni.MachineStatusLink) error {
				// Just copy labels and metadata from MachineStatus resource.
				// This should be safe since Labels are copy-on-write.
				*msl.Metadata().Labels() = *ms.Metadata().Labels()

				msl.TypedSpec().Value.MessageStatus = ms.TypedSpec().Value
				msl.TypedSpec().Value.MachineCreatedAt = ms.Metadata().Created().Unix()

				msl.TypedSpec().Value.TearingDown = ms.Metadata().Phase() == resource.PhaseTearingDown

				if delta, ok := deltas[ms.Metadata().ID()]; ok {
					if msl.TypedSpec().Value.SiderolinkCounter == nil {
						msl.TypedSpec().Value.SiderolinkCounter = &specs.SiderolinkCounterSpec{}
					}

					msl.TypedSpec().Value.SiderolinkCounter.BytesReceived += delta.BytesReceived
					msl.TypedSpec().Value.SiderolinkCounter.BytesSent += delta.BytesSent
					msl.TypedSpec().Value.SiderolinkCounter.LastAlive = pickTime(delta.LastAlive, msl.TypedSpec().Value.SiderolinkCounter.LastAlive)
				}

				return nil
			})
			if err != nil {
				return fmt.Errorf("error creating MachineStatusLink (id: %s) resource: %w", ms.Metadata().ID(), err)
			}
		}

		if err = safe.CleanupOutputs[*omni.MachineStatusLink](ctx, rt); err != nil {
			return fmt.Errorf("error cleaning up MachineStatusLink resources: %w", err)
		}
	}
}

// Almost exact copy from transform.Controller, only as a standalone function.
func cleanupOutputs[T generic.ResourceWithRD](
	ctx context.Context,
	r controller.Runtime,
	hasActiveInput func(T) bool,
) error {
	outputList, err := safe.ReaderListAll[T](ctx, r)
	if err != nil {
		return fmt.Errorf("error listing output resources: %w", err)
	}

	toDelete := xiter.Map(func(r T) resource.Pointer {
		return r.Metadata()
	}, xiter.Filter(func(r T) bool {
		// always attempt clean up of tearing down outputs, even if there is a matching input
		// in the case that output phase is tearing down, while touched is true, actually
		// output belongs to a previous generation of the input resource with the same ID, so the output
		// should be torn down first before the new output is created
		if r.Metadata().Phase() != resource.PhaseTearingDown {
			// this output was touched (has active input), skip it
			if hasActiveInput(r) {
				return false
			}
		}

		return true
	}, outputList.All()))

	_, err = helpers.TeardownAndDestroyAll(ctx, r, toDelete)

	return err
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
