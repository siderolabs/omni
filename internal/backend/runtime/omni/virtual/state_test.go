// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package virtual_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/siderolabs/gen/channel"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"

	virtualres "github.com/siderolabs/omni/client/pkg/omni/resources/virtual"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/virtual"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/virtual/pkg/producers"
)

type mockProducer struct {
	startCh chan struct{}
	stopCh  chan struct{}
	updates chan resource.Resource
	state   state.State
}

func (p *mockProducer) Start() error {
	p.startCh <- struct{}{}

	go func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		for {
			select {
			case <-p.stopCh:
				return
			case res := <-p.updates:
				cur, err := p.state.Get(ctx, res.Metadata())
				if err != nil {
					if !state.IsNotFoundError(err) {
						return
					}

					if err = p.state.Create(ctx, res); err != nil {
						return
					}
				}

				*res.Metadata() = *cur.Metadata()

				if err = p.state.Update(ctx, res); err != nil {
					return
				}
			}
		}
	}()

	return nil
}

func (p *mockProducer) Stop() {
	close(p.stopCh)
}

func (p *mockProducer) Cleanup() {}

func TestComputed(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Second*5)
	defer cancel()

	mp := mockProducer{
		startCh: make(chan struct{}, 1),
		stopCh:  make(chan struct{}, 1),
		updates: make(chan resource.Resource),
	}

	calls := 0

	newProducer := func(ctx context.Context, state state.State, _ resource.Pointer, _ *zap.Logger) (producers.Producer, error) {
		calls++

		if calls > 1 {
			return nil, errors.New("failed to create producer")
		}

		cu := virtualres.NewCurrentUser()
		cu.TypedSpec().Value.Identity = "a@a.com"

		if err := state.Create(ctx, cu); err != nil {
			return nil, err
		}

		mp.state = state

		return &mp, nil
	}

	st := virtual.NewComputed(virtualres.CurrentUserType, newProducer, virtual.NoTransform, time.Second, zaptest.NewLogger(t), false)

	var eg errgroup.Group

	eg.Go(func() error {
		st.Run(ctx)

		return nil
	})

	eg.Go(func() error {
		select {
		case <-time.After(time.Second):
		case <-ctx.Done():
			return ctx.Err()
		}

		cu := virtualres.NewCurrentUser()
		cu.TypedSpec().Value.Identity = "a@b.com"

		if !channel.SendWithContext(ctx, mp.updates, resource.Resource(cu)) {
			return ctx.Err()
		}

		return nil
	})

	require := require.New(t)

	_, err := st.Get(ctx, virtualres.NewCurrentUser().Metadata())
	require.NoError(err)

	events := make(chan state.Event)

	err = st.Watch(ctx, virtualres.NewCurrentUser().Metadata(), events)
	require.NoError(err)

	updated := 0
	created := 0

	for {
		select {
		case <-ctx.Done():
			require.FailNow("timed out waiting for create and update events")
		case event := <-events:
			//nolint:exhaustive
			switch event.Type {
			case state.Created:
				created++
			case state.Updated:
				updated++
			default:
				require.FailNowf("unexpected event %s", event.Type.String())
			}
		}

		if created == 1 && updated == 1 {
			break
		}
	}

	cancel()

	require.NoError(eg.Wait())
}

func TestDeduper(t *testing.T) {
	st := state.WrapCore(namespaced.NewState(inmem.Build))

	ctx, cancel := context.WithTimeout(t.Context(), time.Second*5)
	defer cancel()

	mp := mockProducer{
		startCh: make(chan struct{}, 1),
		stopCh:  make(chan struct{}, 1),
	}

	calls := 0

	newProducer := func(ctx context.Context, state state.State, _ resource.Pointer, _ *zap.Logger) (producers.Producer, error) {
		calls++

		if calls > 1 {
			return nil, errors.New("failed to create producer")
		}

		cu := virtualres.NewCurrentUser()
		cu.TypedSpec().Value.Identity = "a@a.com"

		if err := state.Create(ctx, cu); err != nil {
			return nil, err
		}

		return &mp, nil
	}

	dedup := virtual.NewDedupScheduler(virtualres.CurrentUserType, st, newProducer, time.Millisecond*500, zaptest.NewLogger(t))

	var eg errgroup.Group

	eg.Go(func() error {
		dedup.Run(ctx)

		return nil
	})

	md := virtualres.NewCurrentUser().Metadata()

	require := require.New(t)

	require.NoError(dedup.Start(ctx, md))
	require.NoError(dedup.Start(ctx, md))

	_, err := st.Get(ctx, md)

	require.NoError(err)

	select {
	case <-mp.startCh:
	case <-time.After(time.Second):
		require.FailNow("the producer is not started after 1 second")
	}

	dedup.Stop(md)
	dedup.Stop(md)

	select {
	case <-mp.stopCh:
	case <-ctx.Done():
		require.FailNow("the producer is not stopped")
	}

	cancel()

	require.NoError(eg.Wait())
}
