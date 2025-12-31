// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/task"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/image"
	imagetask "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/task/image"
)

// ImagePullStatusController manages ImagePullStatus resource lifecycle.
type ImagePullStatusController struct {
	runner *task.Runner[imagetask.PullStatusChan, imagetask.PullTaskSpec]

	imageClient image.Client
}

// NewImagePullStatusController creates new ImagePullStatusController.
func NewImagePullStatusController(imageClient image.Client) *ImagePullStatusController {
	return &ImagePullStatusController{
		imageClient: imageClient,
	}
}

// Name implements controller.Controller interface.
func (ctrl *ImagePullStatusController) Name() string {
	return "ImagePullStatusController"
}

// Inputs implements controller.Controller interface.
func (ctrl *ImagePullStatusController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Type:      omni.ImagePullRequestType,
			Kind:      controller.InputWeak,
			Namespace: resources.DefaultNamespace,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *ImagePullStatusController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: omni.ImagePullStatusType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *ImagePullStatusController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) (retErr error) {
	ctrl.runner = task.NewEqualRunner[imagetask.PullTaskSpec]()
	defer ctrl.runner.Stop()

	pullStatusCh := make(chan imagetask.PullStatus)

	for {
		select {
		case <-ctx.Done():
			return nil
		case pullStatus := <-pullStatusCh:
			if err := ctrl.updatePullStatus(ctx, r, pullStatus); err != nil {
				return err
			}
		case <-r.EventCh():
			if err := ctrl.handleEvent(ctx, r, logger, pullStatusCh); err != nil {
				return fmt.Errorf("error handling event: %w", err)
			}
		}

		r.ResetRestartBackoff()
	}
}

func (ctrl *ImagePullStatusController) updatePullStatus(ctx context.Context, r controller.Runtime, pullStatus imagetask.PullStatus) error {
	if err := safe.WriterModify[*omni.ImagePullStatus](ctx, r,
		omni.NewImagePullStatus(pullStatus.Request.Metadata().ID()),
		func(status *omni.ImagePullStatus) error {
			helpers.CopyAllLabels(pullStatus.Request, status)

			status.TypedSpec().Value.LastProcessedNode = pullStatus.Node
			status.TypedSpec().Value.LastProcessedImage = pullStatus.Image
			status.TypedSpec().Value.ProcessedCount = uint32(pullStatus.CurrentNum)
			status.TypedSpec().Value.TotalCount = uint32(pullStatus.TotalNum)

			if pullStatus.Error != nil {
				status.TypedSpec().Value.LastProcessedError = pullStatus.Error.Error()
			} else {
				status.TypedSpec().Value.LastProcessedError = ""
			}

			status.TypedSpec().Value.RequestVersion = pullStatus.Request.Metadata().Version().String()

			return nil
		},
	); err != nil {
		return fmt.Errorf("error updating ImagePullStatus: %w", err)
	}

	return nil
}

func (ctrl *ImagePullStatusController) handleEvent(ctx context.Context, r controller.Runtime, logger *zap.Logger, pullStatusCh chan<- imagetask.PullStatus) error {
	requests, err := safe.ReaderListAll[*omni.ImagePullRequest](ctx, r)
	if err != nil {
		return fmt.Errorf("error listing ImagePullRequest resources: %w", err)
	}

	statusIDToStatus, err := ctrl.getStatusIDToStatus(ctx, r)
	if err != nil {
		return err
	}

	tracker := trackResource(r, resources.DefaultNamespace, omni.ImagePullStatusType)

	expectedPullTasks := map[string]imagetask.PullTaskSpec{}

	for request := range requests.All() {
		tracker.keep(request)

		if request.Metadata().Phase() == resource.PhaseTearingDown {
			continue
		}

		if ctrl.requestIsAlreadyProcessed(statusIDToStatus, request) {
			continue
		}

		expectedPullTasks[request.Metadata().ID()] = imagetask.NewPullTaskSpec(request, ctrl.imageClient)
	}

	ctrl.runner.Reconcile(ctx, logger, expectedPullTasks, pullStatusCh)

	if err = tracker.cleanup(ctx); err != nil { // clean up the orphaned ImagePullStatuses
		return fmt.Errorf("error cleaning up stale ImagePullStatus resources: %w", err)
	}

	return nil
}

func (ctrl *ImagePullStatusController) getStatusIDToStatus(ctx context.Context, r controller.Runtime) (map[resource.ID]*omni.ImagePullStatus, error) {
	statuses, err := safe.ReaderListAll[*omni.ImagePullStatus](ctx, r)
	if err != nil {
		return nil, fmt.Errorf("error listing ImagePullStatus resources: %w", err)
	}

	statusMap := make(map[resource.ID]*omni.ImagePullStatus, statuses.Len())

	for status := range statuses.All() {
		statusMap[status.Metadata().ID()] = status
	}

	return statusMap, nil
}

func (ctrl *ImagePullStatusController) requestIsAlreadyProcessed(statusIDToStatus map[resource.ID]*omni.ImagePullStatus, req *omni.ImagePullRequest) bool {
	sts, ok := statusIDToStatus[req.Metadata().ID()]
	if !ok {
		return false
	}

	stsSpec := sts.TypedSpec().Value

	return stsSpec.RequestVersion == req.Metadata().Version().String() && stsSpec.ProcessedCount == stsSpec.TotalCount
}
