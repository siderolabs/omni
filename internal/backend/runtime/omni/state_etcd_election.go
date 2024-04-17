// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"fmt"
	"path"
	"time"

	"github.com/google/uuid"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/internal/backend/logging"
)

// etcdElections makes sure only one instance of the application is running at a time.
func etcdElections(ctx context.Context, client *clientv3.Client, electionKey string, logger *zap.Logger, f func(ctx context.Context, client *clientv3.Client) error) error {
	logger = logger.With(logging.Component("elections"))

	// use a new context to create a session, as `ctx` might be canceled, and the session is aborted explicitly with Close
	sess, err := concurrency.NewSession(client, concurrency.WithContext(context.Background())) //nolint:contextcheck
	if err != nil {
		return fmt.Errorf("failed to create concurrency session: %w", err)
	}
	defer sess.Close() //nolint:errcheck

	// create a random key for this campaign, so there will be no way to "resume" the elections, as there is no stable ID
	campaignKey := uuid.NewString()

	election := concurrency.NewElection(sess, path.Join(electionKey, "election"))

	campaignErrCh := make(chan error)

	go func() {
		campaignErrCh <- election.Campaign(ctx, campaignKey)
	}()

	logger.Info("running the etcd election campaign")

	// wait for the etcd election campaign to be complete
campaignLoop:
	for {
		select {
		case err = <-campaignErrCh:
			if err != nil {
				return fmt.Errorf("failed to conduct campaign: %w", err)
			}

			// won the election campaign!
			break campaignLoop
		case <-sess.Done():
			logger.Info("etcd session closed")

			return nil
		case <-ctx.Done():
			return nil
		}
	}

	logger.Info("won the etcd election campaign")

	defer func() { //nolint:contextcheck
		// use a new context to resign, as `ctx` might be canceled
		resignCtx, resignCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer resignCancel()

		resignErr := election.Resign(resignCtx)

		logger.Info("resigned from the etcd election campaign", zap.Error(resignErr))
	}()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		observe := election.Observe(ctx)

	observeLoop:
		for {
			select {
			case <-sess.Done():
				logger.Info("etcd session closed")

				break observeLoop
			case <-ctx.Done():
				break observeLoop
			case resp, ok := <-observe:
				if !ok {
					break observeLoop
				}

				if string(resp.Kvs[0].Value) != campaignKey {
					logger.Info("detected new leader", zap.ByteString("leader", resp.Kvs[0].Value))

					break observeLoop
				}
			}
		}
	}()

	return f(ctx, client)
}
