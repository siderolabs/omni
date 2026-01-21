// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		Embedded:       true,
		EmbeddedDBPath: t.TempDir(),
		Endpoints:      []string{"http://localhost:0"},
		RunElections:   true,
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
		Embedded:       true,
		EmbeddedDBPath: t.TempDir(),
		Endpoints:      []string{"http://localhost:0"},
		RunElections:   true,
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
