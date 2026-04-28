// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"errors"
	"fmt"
	"path"
	"sync"
	"time"

	"github.com/google/uuid"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/panichandler"
	"github.com/siderolabs/omni/internal/backend/logging"
)

type etcdElections struct {
	logger         *zap.Logger
	election       *concurrency.Election
	session        *concurrency.Session
	eg             *panichandler.ErrGroup
	campaignCancel context.CancelFunc

	mu         sync.Mutex
	campaignWg sync.WaitGroup
}

func newEtcdElections(logger *zap.Logger) *etcdElections {
	return &etcdElections{
		logger: logger.With(logging.Component("etcd_elections")),
		eg:     panichandler.NewErrGroup(),
	}
}

func (ee *etcdElections) createSession(ctx context.Context, client *clientv3.Client, electionKey string) error {
	ee.mu.Lock()
	defer ee.mu.Unlock()

	var err error
	// use a new context to create a session, as `ctx` might be canceled, and the session is aborted explicitly with Close
	ee.session, err = concurrency.NewSession(client, concurrency.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("failed to create concurrency session: %w", err)
	}

	ee.election = concurrency.NewElection(ee.session, path.Join(electionKey, "election"))

	return nil
}

func (ee *etcdElections) run(ctx context.Context, client *clientv3.Client, electionKey string, errChan chan<- error) error {
	var err error
	if err = ee.createSession(ctx, client, electionKey); err != nil {
		return err
	}

	// create a random key for this campaign, so there will be no way to "resume" the elections, as there is no stable ID
	campaignKey := uuid.NewString()

	campaignErrCh := make(chan error, 1)

	// Run Campaign on a sub-context so stop() can cancel it and wait for the
	// goroutine to exit before calling Resign(); this avoids the data race on
	// the Election's internal fields (leaderKey, leaderSession).
	campaignCtx, campaignCancel := context.WithCancel(ctx)

	ee.mu.Lock()
	ee.campaignCancel = campaignCancel
	ee.campaignWg.Add(1)
	ee.mu.Unlock()

	panichandler.Go(func() {
		defer ee.campaignWg.Done()

		ee.mu.Lock()
		election := ee.election
		ee.mu.Unlock()

		campaignErrCh <- election.Campaign(campaignCtx, campaignKey)
	}, ee.logger)

	ee.logger.Info("running the etcd election campaign")

campaignLoop:
	for {
		select {
		case err = <-campaignErrCh:
			if err != nil {
				// Campaign was canceled (parent ctx or stop()) — clean exit, not a campaign failure.
				if errors.Is(err, context.Canceled) {
					return nil
				}

				return fmt.Errorf("failed to conduct campaign: %w", err)
			}

			// won the election campaign!
			break campaignLoop
		case <-ee.session.Done():
			ee.logger.Info("etcd session closed")

			return nil
		case <-ctx.Done():
			return nil
		}
	}

	// wait for the etcd election campaign to be complete

	ee.logger.Info("won the etcd election campaign")

	ee.eg.Go(func() error {
		observe := ee.election.Observe(ctx)

		for {
			select {
			case <-ee.session.Done():
				ee.logger.Error("etcd session closed")

				return nil
			case <-ctx.Done():
				return ctx.Err()
			case resp, ok := <-observe:
				if !ok {
					ee.logger.Error("etcd observe channel closed")

					return nil
				}

				if string(resp.Kvs[0].Value) != campaignKey {
					ee.logger.Error("detected new leader", zap.ByteString("leader", resp.Kvs[0].Value))

					errChan <- errors.New("etcd detected new leader")

					return errors.New("etcd detected new leader")
				}
			}
		}
	})

	return nil
}

func (ee *etcdElections) stop() error {
	ee.mu.Lock()
	cancel := ee.campaignCancel
	ee.mu.Unlock()

	// Cancel the in-flight Campaign and wait for the goroutine to exit. After
	// this point, no one is mutating the Election's internal fields, so it is
	// safe to call Resign() alongside session.Close() without racing Campaign().
	if cancel != nil {
		cancel()
	}

	ee.campaignWg.Wait()

	ee.mu.Lock()
	defer ee.mu.Unlock()

	if ee.election != nil {
		// Use a fresh context: the caller ctx is typically canceled at shutdown,
		// and we still want to delete the leader key promptly. Resign() issues a
		// transactional delete of the leader key independent of the lease Revoke
		// performed by session.Close(), so the leader key is removed even if
		// Revoke is dropped during a racy shutdown.
		resignCtx, resignCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer resignCancel()

		resignErr := ee.election.Resign(resignCtx)
		ee.logger.Info("resigned from the etcd election campaign", zap.Error(resignErr))
	}

	if ee.session != nil {
		if err := ee.session.Close(); err != nil {
			return err
		}
	}

	return ee.eg.Wait()
}
