// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	xmaps "github.com/siderolabs/gen/maps"
	"github.com/siderolabs/gen/xiter"
	"github.com/siderolabs/gen/xslices"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/cosi/helpers"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	ctrlhelpers "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
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
		safe.Input[*siderolink.Link](controller.InputWeak),
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

		links, err := safe.ReaderListAll[*siderolink.Link](ctx, rt)
		if err != nil {
			return fmt.Errorf("error listing SideroLink resources: %w", err)
		}

		machineStatusMap := xslices.ToMap(slices.Collect(msList.All()), func(r *omni.MachineStatus) (resource.ID, *omni.MachineStatus) {
			return r.Metadata().ID(), r
		})

		linkMap := xslices.ToMap(slices.Collect(links.All()), func(r *siderolink.Link) (resource.ID, *siderolink.Link) {
			return r.Metadata().ID(), r
		})

		finalIDs := make([]resource.ID, 0, len(machineStatusMap)+len(linkMap))

		finalIDs = append(finalIDs, xmaps.Keys(machineStatusMap)...)
		finalIDs = append(finalIDs, xmaps.Keys(linkMap)...)
		finalIDsSet := xslices.ToSet(finalIDs)

		for id := range finalIDsSet {
			ms, msOK := machineStatusMap[id]
			link, linkOK := linkMap[id]

			switch {
			// Skip creating MachineStatusLink resources for machines that are being torn down.
			case msOK && ms.Metadata().Phase() == resource.PhaseTearingDown && !linkOK:
				continue
			// Create or update MachineStatusLink resource.
			case msOK:
				if err = ctrl.createMachineStatusLink(ctx, rt, ms, deltas); err != nil {
					return err
				}
			// Create a placeholder MachineStatusLink resource for links that are being torn down.
			case linkOK && link.Metadata().Phase() == resource.PhaseTearingDown:
				if err = ctrl.keepTearingDownMachineStatusLink(ctx, rt, link, deltas); err != nil {
					return err
				}
			}
		}

		if err = safe.CleanupOutputs[*omni.MachineStatusLink](ctx, rt); err != nil {
			return fmt.Errorf("error cleaning up MachineStatusLink resources: %w", err)
		}
	}
}

func (ctrl *MachineStatusLinkController) createMachineStatusLink(ctx context.Context, rt controller.Runtime, ms *omni.MachineStatus, deltas siderolinkmanager.LinkCounterDeltas) error {
	emptyResource := omni.NewMachineStatusLink(ms.Metadata().ID())

	err := safe.WriterModify(ctx, rt, emptyResource, func(msl *omni.MachineStatusLink) error {
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

	return nil
}

func (ctrl *MachineStatusLinkController) keepTearingDownMachineStatusLink(ctx context.Context, rt controller.Runtime, link *siderolink.Link, deltas siderolinkmanager.LinkCounterDeltas) error {
	emptyResource := omni.NewMachineStatusLink(link.Metadata().ID())

	err := safe.WriterModify(ctx, rt, emptyResource, func(msl *omni.MachineStatusLink) error {
		ctrlhelpers.CopyLabels(msl, link)

		msl.TypedSpec().Value.TearingDown = true

		if delta, ok := deltas[link.Metadata().ID()]; ok {
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
		return fmt.Errorf("error creating MachineStatusLink (id: %s) resource: %w", link.Metadata().ID(), err)
	}

	return nil
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
