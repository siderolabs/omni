// Copyright (c) 2025 Sidero Labs, Inc.
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
	logger   *zap.Logger
	election *concurrency.Election
	session  *concurrency.Session
	eg       *panichandler.ErrGroup

	mu sync.Mutex
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

	campaignErrCh := make(chan error)

	panichandler.Go(func() {
		campaignErrCh <- ee.election.Campaign(ctx, campaignKey)
	}, ee.logger)

	ee.logger.Info("running the etcd election campaign")

campaignLoop:
	for {
		select {
		case err = <-campaignErrCh:
			if err != nil {
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
	defer ee.mu.Unlock()

	if ee.election != nil {
		// use a new context to resign, as `ctx` might be canceled
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
