// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package sequence provides a generic implementation of a QController which processes a sequence of stages in order.
package sequence

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/siderolabs/gen/xerrors"
	"go.uber.org/zap"
)

// Controller is a QController that processes a sequence of stages in order.
type Controller[Input, Output generic.ResourceWithRD] struct {
	*qtransform.QController[Input, Output]
	sequencer Sequencer[Input, Output]
}

// NewController creates a new QController configured to run through the stages defined in the Sequencer. It handles stage lookup, execution, and error management (including requeueing).
func NewController[Input, Output generic.ResourceWithRD](name string, sequencer Sequencer[Input, Output]) *Controller[Input, Output] {
	ctrl := &Controller[Input, Output]{
		sequencer: sequencer,
	}

	ctrl.QController = qtransform.NewQController(
		qtransform.Settings[Input, Output]{
			Name:              name,
			MapMetadataFunc:   sequencer.MapFunc,
			UnmapMetadataFunc: sequencer.UnmapFunc,
			TransformExtraOutputFunc: func(ctx context.Context, r controller.ReaderWriter, logger *zap.Logger, input Input, output Output) error {
				stages := sequencer.Stages()
				sequenceContext := NewContext(r, input, output, stages)

				if len(sequenceContext.stages) == 0 {
					logger.Error("no stages defined")

					return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("no stages defined")
				}

				remainingStages, err := sequenceContext.RemainingStages()
				if err != nil {
					return fmt.Errorf("failed to get stage to run: %w", err)
				}

				for idx, stage := range remainingStages {
					logger.Info("processing stage", zap.String("stage", stage.Name()), zap.Int("remaining", len(remainingStages)-idx))

					processNextStage, stageErr := stage.Run(ctx, logger, sequenceContext)
					if stageErr != nil {
						return fmt.Errorf("failed to process stage %s: %w", stage.Name(), stageErr)
					}

					if !processNextStage {
						break
					}
				}

				return nil
			},
			FinalizerRemovalExtraOutputFunc: func(ctx context.Context, r controller.ReaderWriter, logger *zap.Logger, input Input) error {
				c, ok := sequencer.(Cleaner[Input])
				if !ok {
					return nil
				}

				return c.FinalizerRemoval(ctx, r, logger, input)
			},
		},
		sequencer.Options()...,
	)

	return ctrl
}
