// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/omni/internal/backend/runtime/omni"
	"github.com/siderolabs/omni/internal/pkg/config"
)

func mockRunner(id int, started chan<- int, closed <-chan error) func(ctx context.Context, client *clientv3.Client) error {
	return func(ctx context.Context, _ *clientv3.Client) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case started <- id:
		}

		select {
		case err := <-closed:
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func TestEtcdElections(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	logger := zaptest.NewLogger(t)

	require.NoError(t, omni.GetEmbeddedEtcdClient(ctx, &config.EtcdParams{
		Embedded:       true,
		EmbeddedDBPath: t.TempDir(),
		Endpoints:      []string{"http://localhost:0"},
	}, logger, func(ctx context.Context, client *clientv3.Client) error {
		started := make(chan int)
		closed := make(chan error)
		errCh := make(chan error)
		electionKey := uuid.NewString()

		// run the first mock runnner, it should win the elections and keep running
		go func() {
			errCh <- omni.EtcdElections(ctx, client, electionKey, logger, mockRunner(1, started, closed))
		}()

		select {
		case id := <-started:
			require.Equal(t, 1, id)
		case <-ctx.Done():
			t.Fatal("first runner didn't start")
		}

		// run the second mock runner, it should not start as the elections are already won
		go func() {
			errCh <- omni.EtcdElections(ctx, client, electionKey, logger, mockRunner(2, started, closed))
		}()

		select {
		case <-started:
			t.Fatal("shouldn't start second runner")
		case <-time.After(time.Second):
		}

		// stop the first runner, the second runner should start
		select {
		case closed <- nil:
		case <-ctx.Done():
			t.Fatal("first runner didn't stop")
		}

		select {
		case err := <-errCh:
			require.NoError(t, err)
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
		case closed <- errors.New("stopped"):
		case <-ctx.Done():
			t.Fatal("second runner didn't stop")
		}

		select {
		case err := <-errCh:
			require.Error(t, err)
			require.EqualError(t, err, "stopped")
		case <-ctx.Done():
			t.Fatal("second runner didn't stop")
		}

		return nil
	}))
}
