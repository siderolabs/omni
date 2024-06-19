// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package panichandler_test

import (
	"errors"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/siderolabs/omni/client/pkg/panichandler"
)

func TestGoNoPanic(t *testing.T) {
	logObserverCore, observedLogs := observer.New(zapcore.InfoLevel)
	logger := zap.New(logObserverCore)

	var wg sync.WaitGroup

	wg.Add(1)

	panichandler.Go(func() {
		wg.Done()
	}, logger)

	wg.Wait()

	assert.Empty(t, observedLogs.All())
}

func TestGoLogPanic(t *testing.T) {
	currentFile, currentFunc := trace()

	logObserverCore, observedLogs := observer.New(zapcore.InfoLevel)
	logger := zap.New(logObserverCore)

	panichandler.Go(func() {
		panic("test")
	}, logger)

	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		if assert.Equal(collect, 1, observedLogs.Len()) {
			loggedEntry := observedLogs.All()[0]
			stack := loggedEntry.ContextMap()["stack"]

			t.Logf("log msg: %s stack: %s", loggedEntry.Message, stack)

			assert.Equal(collect, zapcore.DPanicLevel, loggedEntry.Level)
			assert.Contains(collect, loggedEntry.Message, panichandler.ErrPanic.Error())

			// assert stack trace
			assert.Contains(collect, stack, currentFunc)
			assert.Contains(collect, stack, currentFile)
		}
	}, 1*time.Second, 10*time.Millisecond)
}

func TestRunErrFErrUnchanged(t *testing.T) {
	expectedErr := errors.New("test error")

	eg := panichandler.NewErrGroup()

	eg.Go(func() error {
		return expectedErr
	})

	err := eg.Wait()

	assert.Equal(t, expectedErr, err)
}

func TestRunErrFHandlePanic(t *testing.T) {
	currentFile, currentFunc := trace()

	eg := panichandler.NewErrGroup()

	eg.Go(func() error {
		return nil // no err, all good
	})

	eg.Go(func() error {
		panic("test")
	})

	err := eg.Wait()

	t.Logf("error: %v", err)

	assert.ErrorIs(t, err, panichandler.ErrPanic)

	// assert stack trace
	assert.ErrorContains(t, err, currentFunc)
	assert.ErrorContains(t, err, currentFile)
}

// trace returns the file and function name of the caller.
func trace() (file, function string) {
	pc := make([]uintptr, 15)
	n := runtime.Callers(2, pc)
	frames := runtime.CallersFrames(pc[:n])
	frame, _ := frames.Next()

	return frame.File, frame.Function
}
