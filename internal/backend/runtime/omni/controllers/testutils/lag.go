// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package testutils

import (
	"context"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"

	omniruntime "github.com/siderolabs/omni/internal/backend/runtime/omni"
)

// LaggingStateBuilder returns a state builder which delivers the watch events of the given resource types one by one,
// each delayed by the given duration, so the controller runtime's cached copies of them lag behind the state.
//
// The delayed events pile up in the inmem watch history (1024 per type), so this is for tests with a handful of them.
func LaggingStateBuilder(delay time.Duration, types ...resource.Type) namespaced.StateBuilder {
	inner := omniruntime.TestStateBuilder()

	lagging := map[resource.Type]struct{}{}
	for _, typ := range types {
		lagging[typ] = struct{}{}
	}

	return func(ns resource.Namespace) state.CoreState {
		return &laggingState{CoreState: inner(ns), delay: delay, lagging: lagging}
	}
}

type laggingState struct {
	state.CoreState

	lagging map[resource.Type]struct{}
	delay   time.Duration
}

func (st *laggingState) WatchKindAggregated(ctx context.Context, kind resource.Kind, ch chan<- []state.Event, opts ...state.WatchKindOption) error {
	if _, ok := st.lagging[kind.Type()]; !ok {
		return st.CoreState.WatchKindAggregated(ctx, kind, ch, opts...)
	}

	direct := make(chan []state.Event)

	if err := st.CoreState.WatchKindAggregated(ctx, kind, direct, opts...); err != nil {
		return err
	}

	go func() {
		for {
			var events []state.Event

			select {
			case <-ctx.Done():
				return
			case events = <-direct:
			}

			for _, event := range events {
				select {
				case <-ctx.Done():
					return
				case <-time.After(st.delay):
				}

				select {
				case <-ctx.Done():
					return
				case ch <- []state.Event{event}:
				}
			}
		}
	}()

	return nil
}
