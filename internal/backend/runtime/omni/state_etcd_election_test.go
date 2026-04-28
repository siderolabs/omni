// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"context"
	"path"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"

	"github.com/siderolabs/omni/internal/backend/runtime/omni"
	"github.com/siderolabs/omni/internal/pkg/config"
)

func mockRunner(ctx context.Context, id int, started chan<- int, closed <-chan struct{}) {
	select {
	case <-ctx.Done():
		return
	case started <- id:
	}

	select {
	case <-closed:
		return
	case <-ctx.Done():
		return
	}
}

func TestEtcdElectionsLost(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	logger := zaptest.NewLogger(t)

	state, err := omni.GetEmbeddedEtcdClientWithServer(&config.EtcdParams{
		Embedded:       new(true),
		EmbeddedDBPath: new(t.TempDir()),
		Endpoints:      []string{"http://localhost:0"},
		RunElections:   new(true),
	}, logger)

	require.NoError(t, err)

	started := make(chan int)
	errCh := make(chan error, 1)
	electionKey := uuid.NewString()

	var wg sync.WaitGroup

	defer wg.Wait()

	// run mock runner, it should win the elections and keep running
	wg.Go(func() {
		errCh <- state.RunElections(ctx, electionKey, logger)

		defer state.StopElections(electionKey) //nolint:errcheck

		mockRunner(ctx, 1, started, nil)
	})

	select {
	case id := <-started:
		require.Equal(t, 1, id)
	case <-ctx.Done():
		t.Fatal("runner didn't start")
	}

	// abort etcd, that aborts the election campaign
	assert.NoError(t, state.Close())

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-ctx.Done():
		t.Fatal("runner didn't stop")
	}
}

func TestEtcdElections(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	logger := zaptest.NewLogger(t)

	state, err := omni.GetEmbeddedEtcdClientWithServer(&config.EtcdParams{
		Embedded:       new(true),
		EmbeddedDBPath: new(t.TempDir()),
		Endpoints:      []string{"http://localhost:0"},
		RunElections:   new(true),
	}, logger)

	require.NoError(t, err)

	started := make(chan int, 2)
	closed := make(chan struct{})
	electionKey := uuid.NewString()

	var eg errgroup.Group

	// run the first mock runnner, it should win the elections and keep running
	eg.Go(func() error {
		e := state.RunElections(ctx, electionKey, logger)

		defer state.StopElections(electionKey) //nolint:errcheck

		mockRunner(ctx, 1, started, closed)

		return e
	})

	select {
	case id := <-started:
		require.Equal(t, 1, id)
	case <-ctx.Done():
		t.Fatal("first runner didn't start")
	}

	// run the second mock runner, it should not start as the elections are already won
	eg.Go(func() error {
		e := state.RunElections(ctx, electionKey, logger)

		defer state.StopElections(electionKey) //nolint:errcheck

		mockRunner(ctx, 2, started, closed)

		return e
	})

	select {
	case <-started:
		t.Fatal("shouldn't start second runner")
	case <-time.After(time.Second):
	}

	// stop the first runner, the second runner should start
	select {
	case closed <- struct{}{}:
	case <-ctx.Done():
		t.Fatal("first runner didn't stop")
	}

	select {
	case id := <-started:
		require.Equal(t, 2, id)
	case <-ctx.Done():
		t.Fatal("second runner didn't start")
	}

	// stop the second runner, it should terminate
	select {
	case closed <- struct{}{}:
	case <-ctx.Done():
		t.Fatal("second runner didn't stop")
	}

	require.NoError(t, eg.Wait())
}

// TestEtcdElectionsLeaderKeyRemovedAfterCtxCancel verifies that StopElections
// removes the leader key from etcd immediately even when the context that was
// passed to RunElections has already been canceled (the typical shutdown order:
// signal handler cancels the app ctx, then defers run state.Close()).
//
// Regression: 1.7.0 dropped the explicit Election.Resign call from stop() and
// relied solely on session.Close() → lease Revoke. The session was created with
// concurrency.WithContext(ctx), so once ctx is canceled the session's keepalive
// goroutine exits and the session is "Done"; session.Close()'s Revoke is then
// a best-effort RPC that can be dropped, leaving the leader key alive until the
// lease TTL (default 60s) expires. The pre-1.7.0 code Txn-deleted the leader
// key via Resign with a fresh background ctx, independent of lease revoke.
func TestEtcdElectionsLeaderKeyRemovedAfterCtxCancel(t *testing.T) {
	logger := zaptest.NewLogger(t)

	state, err := omni.GetEmbeddedEtcdClientWithServer(&config.EtcdParams{
		Embedded:       new(true),
		EmbeddedDBPath: new(t.TempDir()),
		Endpoints:      []string{"http://localhost:0"},
		RunElections:   new(true),
	}, logger)
	require.NoError(t, err)

	t.Cleanup(func() { assert.NoError(t, state.Close()) })

	electionKey := uuid.NewString()
	leaderPrefix := path.Join(electionKey, "election")

	runCtx, runCancel := context.WithCancel(t.Context())
	require.NoError(t, state.RunElections(runCtx, electionKey, logger))

	resp, err := state.Client().Get(t.Context(), leaderPrefix, clientv3.WithPrefix())
	require.NoError(t, err)
	require.NotEmpty(t, resp.Kvs, "leader key should exist after winning the election")

	// Simulate shutdown: cancel the run ctx first. This is what the signal
	// handler does in production before state.Close() runs.
	runCancel()

	// StopElections may return context.Canceled propagated from the Observe
	// goroutine — that is unrelated to lease release and is ignored by
	// etcdState.Close in production. The contract we verify is that the leader
	// key is gone.
	if err = state.StopElections(electionKey); err != nil {
		require.ErrorIs(t, err, context.Canceled)
	}

	resp, err = state.Client().Get(t.Context(), leaderPrefix, clientv3.WithPrefix())
	require.NoError(t, err)
	require.Empty(t, resp.Kvs, "leader key should be removed immediately after StopElections, not after lease TTL — see go.etcd.io/etcd/client/v3/concurrency.WithContext docstring")
}
