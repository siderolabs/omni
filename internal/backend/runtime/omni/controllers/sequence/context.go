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
type Context[Input generic.ResourceWithRD, Output generic.ResourceWithRD] struct {
	Runtime controller.ReaderWriter
	Input   Input
	Output  Output
	stages  []Stage[Input, Output]
}

// NewContext creates a new Context.
func NewContext[Input, Output generic.ResourceWithRD](runtime controller.ReaderWriter, input Input, output Output, stages []Stage[Input, Output]) Context[Input, Output] {
	return Context[Input, Output]{Runtime: runtime, Input: input, Output: output, stages: stages}
}

// RemainingStages returns the stages that are yet to be processed in the sequence.
func (c Context[Input, Output]) RemainingStages() ([]Stage[Input, Output], error) {
	idx, err := c.getStageIndex()
	if err != nil {
		return nil, fmt.Errorf("failed to get stage index: %w", err)
	}

	return c.stages[idx:], nil
}

func (c Context[Input, Output]) getStageIndex() (int, error) {
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

func (c Context[Input, Output]) incrementStageIndex() error {
	idx, err := c.getStageIndex()
	if err != nil {
		return fmt.Errorf("failed to get stage index: %w", err)
	}

	nextIdx := (idx + 1) % len(c.stages)
	c.Output.Metadata().Annotations().Set(sequencedStageIndex, strconv.Itoa(nextIdx))

	return nil
}
