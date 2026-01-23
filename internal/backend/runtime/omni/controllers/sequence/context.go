// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package sequence

import (
	"fmt"
	"strconv"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

const sequencedStageIndex = omni.SystemLabelPrefix + "sequenced-stage-index"

// Context stores the runtime, input and output resources of a sequence.
type Context[I generic.ResourceWithRD, O generic.ResourceWithRD] struct {
	Runtime controller.ReaderWriter
	Input   I
	Output  O
	stages  []Stage[I, O]
}

// NewContext creates a new Context.
func NewContext[I, O generic.ResourceWithRD](runtime controller.ReaderWriter, input I, output O, stages []Stage[I, O]) Context[I, O] {
	return Context[I, O]{Runtime: runtime, Input: input, Output: output, stages: stages}
}

// StageToRun returns the stage that should be run next.
func (c Context[I, O]) StageToRun() (*Stage[I, O], error) {
	idx, err := c.getStageIndex()
	if err != nil {
		return nil, fmt.Errorf("failed to get stage index: %w", err)
	}

	return &c.stages[idx], nil
}

func (c Context[I, O]) getStageIndex() (int, error) {
	stageIdx, ok := c.Output.Metadata().Annotations().Get(sequencedStageIndex)
	if ok {
		stageIndex, err := strconv.Atoi(stageIdx)
		if err != nil {
			return -1, fmt.Errorf("failed to parse stage index: %w", err)
		}

		if stageIndex < 0 || stageIndex >= len(c.stages) {
			return -1, fmt.Errorf("stage index %d out of bounds (total stages: %d)", stageIndex, len(c.stages))
		}

		return stageIndex, nil
	}

	// Stage index isn't set, start from the beginning
	return 0, nil
}

func (c Context[I, O]) incrementStageIndex() error {
	idx, err := c.getStageIndex()
	if err != nil {
		return fmt.Errorf("failed to get stage index: %w", err)
	}

	nextIdx := (idx + 1) % len(c.stages)
	c.Output.Metadata().Annotations().Set(sequencedStageIndex, strconv.Itoa(nextIdx))

	return nil
}
