// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package task_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/task"
)

func TestRunner(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	assert := assert.New(t)

	in := &taskInputMock{
		commandCh: make(chan taskCommand),
	}

	assertTask := func(id string, expectedRunning bool) {
		assert.Eventually(func() bool {
			running, _ := in.runningTasks.Get(id)

			return running == expectedRunning
		}, time.Second, time.Millisecond)
	}

	runner := task.NewRunner(func(a, b taskSpec) bool {
		return a == b
	})

	runner.Reconcile(ctx, logger, nil, in)

	runner.Reconcile(ctx, logger, map[task.ID]taskSpec{
		"task1": "task1",
		"task2": "task2",
	}, in)

	assertTask("task1", true)
	assertTask("task2", true)

	runner.Reconcile(ctx, logger, map[task.ID]taskSpec{
		"task2": "task2",
	}, in)

	assertTask("task1", false)
	assertTask("task2", true)

	runner.Reconcile(ctx, logger, map[task.ID]taskSpec{
		"task2": "task3", // a bit of hack with different IDs to test the replace logic
		"task4": "task4",
	}, in)

	assertTask("task2", false)
	assertTask("task3", true)
	assertTask("task4", true)

	runner.Stop()

	assertTask("task3", false)
	assertTask("task4", false)
}
