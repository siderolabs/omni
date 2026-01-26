// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package sequence

import (
	"context"
	"errors"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"go.uber.org/zap"
)

// RunFunc is the function signature for a stage's run function.
type RunFunc[Input, Output generic.ResourceWithRD] func(ctx context.Context, logger *zap.Logger, sequenceContext Context[Input, Output]) error

// NewStage creates a new Stage with provided name and run function.
//
// The run function should return an error if any occurred during execution. A nil value indicates that the stage is completed and the controller will move to the next stage.
// If the stage is not yet complete, an error of type ErrWait should be returned.
func NewStage[Input, Output generic.ResourceWithRD](name string, run RunFunc[Input, Output],
) Stage[Input, Output] {
	return Stage[Input, Output]{name: name, run: run}
}

// Stage implements a single stage in a sequence of stages.
type Stage[Input, Output generic.ResourceWithRD] struct {
	run  RunFunc[Input, Output]
	name string
}

// Name of the stage.
func (s Stage[Input, Output]) Name() string {
	return s.name
}

// ErrWait is a special error indicating that the stage is not yet complete and needs to be processed again later. However, unlike other QController errors, it does not prevent changes to Output from
// being persisted.
// It can also be used by the initial stage as a break condition to prevent further stages from running.
var ErrWait = errors.New("wait for stage completion")

// Run the stage. If the stage is completed successfully, it returns true to indicate that the controller should proceed to the next stage.
// If the stage is not yet complete and needs to be retried later, it returns false and a nil error.
// If any other error occurs during execution, it returns false and the error.
func (s Stage[Input, Output]) Run(ctx context.Context, logger *zap.Logger, sequenceContext Context[Input, Output]) (processNextStage bool, err error) {
	if err = s.run(ctx, logger, sequenceContext); err != nil {
		if errors.Is(err, ErrWait) {
			return false, nil
		}

		return false, err
	}

	if err = sequenceContext.incrementStageIndex(); err != nil {
		return false, fmt.Errorf("failed to increment stage index: %w", err)
	}

	return true, nil
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
