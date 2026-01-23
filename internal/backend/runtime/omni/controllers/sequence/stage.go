// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package sequence

import (
	"context"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"go.uber.org/zap"
)

// NewStage creates a new Stage with provided name and run function.
//
// The run function should return a boolean indicating whether the stage is completed and an error if any occurred during execution.
func NewStage[Input, Output generic.ResourceWithRD](name string, run func(ctx context.Context, logger *zap.Logger, sequenceContext Context[Input, Output],
) (completed bool, err error),
) Stage[Input, Output] {
	return Stage[Input, Output]{name: name, run: run}
}

// Stage implements a single stage in a sequence of stages.
type Stage[Input, Output generic.ResourceWithRD] struct {
	run  func(ctx context.Context, logger *zap.Logger, sequenceContext Context[Input, Output]) (completed bool, err error)
	name string
}

// Name of the stage.
func (s Stage[Input, Output]) Name() string {
	return s.name
}

// Run the stage. If the stage is completed, it increments the stage index in the sequence context.
func (s Stage[Input, Output]) Run(ctx context.Context, logger *zap.Logger, sequenceContext Context[Input, Output]) error {
	completed, err := s.run(ctx, logger, sequenceContext)
	if completed {
		if indexErr := sequenceContext.incrementStageIndex(); indexErr != nil {
			logger.Error("failed to increment stage index", zap.Error(indexErr))

			return indexErr
		}
	}

	return err
}

// Sequencer is the interface that should be implemented to utilize the sequence controller.
type Sequencer[Input, Output generic.ResourceWithRD] interface {
	MapFunc(Input) Output
	UnmapFunc(Output) Input
	Options() []qtransform.ControllerOption
	Stages() []Stage[Input, Output]
}

// Cleaner is the interface that should be implemented to clean up resources when Input is tearing down.
type Cleaner[Input generic.ResourceWithRD] interface {
	FinalizerRemoval(ctx context.Context, r controller.ReaderWriter, logger *zap.Logger, input Input) error
}
