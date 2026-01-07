// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package testutils

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/cosi-project/runtime/pkg/controller/runtime"
	"github.com/cosi-project/runtime/pkg/controller/runtime/options"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup" //nolint:depguard // this is only used in tests

	omniruntime "github.com/siderolabs/omni/internal/backend/runtime/omni"
)

type DynamicStateBuilder struct { //nolint:govet
	mx sync.Mutex
	M  map[resource.Namespace]state.CoreState
}

func (b *DynamicStateBuilder) Builder(ns resource.Namespace) state.CoreState {
	b.mx.Lock()
	defer b.mx.Unlock()

	if s, ok := b.M[ns]; ok {
		return s
	}

	s := inmem.Build(ns)

	b.M[ns] = s

	return s
}

func (b *DynamicStateBuilder) Set(ns resource.Namespace, state state.CoreState) {
	b.mx.Lock()
	defer b.mx.Unlock()

	if _, ok := b.M[ns]; ok {
		panic(fmt.Errorf("state for namespace %s already exists", ns))
	}

	b.M[ns] = state
}

type TestOptions struct {
	StateBuilder namespaced.StateBuilder
	LogLevel     *zapcore.Level
	DisableCache bool
}

// TestContext is a test helper struct that provides the state and the runtime to the test.
type TestContext struct {
	State   state.State
	Runtime *runtime.Runtime
	FailCh  chan error
	Logger  *zap.Logger
}

// TestFunc is a test helper function that provides the state and the runtime to the test.
type TestFunc func(ctx context.Context, testContext TestContext)

// WithRuntime is a test helper function that starts the COSI runtime with the provided beforeStart and afterStart functions.
//
// beforeStart can be used to register the controllers and other do other preparation work before the runtime starts.
//
// afterStart can be used to do the actual assertions on the controllers' expected behavior after the runtime has started.
func WithRuntime(ctx context.Context, t *testing.T, testOptions TestOptions, beforeStart, afterStart TestFunc) {
	ctx, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)

	if testOptions.StateBuilder == nil {
		testOptions.StateBuilder = inmem.Build
	}

	var loggerOpts []zaptest.LoggerOption
	if testOptions.LogLevel != nil {
		loggerOpts = append(loggerOpts, zaptest.Level(*testOptions.LogLevel))
	}

	logger := zaptest.NewLogger(t, loggerOpts...).With(zap.String("test", t.Name()))
	st := state.WrapCore(namespaced.NewState(testOptions.StateBuilder))

	var opts []options.Option

	if !testOptions.DisableCache {
		opts = append(opts, omniruntime.RuntimeCacheOptions()...)
	}

	cosiRuntime, err := runtime.NewRuntime(st, logger, opts...)

	require.NoError(t, err)

	eg, ctx := errgroup.WithContext(ctx)

	t.Cleanup(func() {
		require.NoError(t, eg.Wait(), "errgroup failed")
	})

	testContext := TestContext{
		State:   st,
		Runtime: cosiRuntime,
		FailCh:  make(chan error),
		Logger:  logger,
	}

	eg.Go(func() error {
		select {
		case failErr := <-testContext.FailCh:
			return failErr
		case <-ctx.Done():
			return nil
		}
	})

	beforeStart(ctx, testContext)

	eg.Go(func() error {
		logger.Debug("start runtime")
		defer logger.Info("runtime stopped")

		return cosiRuntime.Run(ctx)
	})

	afterStart(ctx, testContext)

	cancel()

	logger.Info("context canceled, wait for the runtime to stop")
}
