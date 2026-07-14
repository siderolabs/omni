// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package grpc_test

import (
	"context"
	"net"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	"github.com/siderolabs/omni/client/api/omni/management"
	"github.com/siderolabs/omni/client/pkg/access/role"
	managementcli "github.com/siderolabs/omni/client/pkg/client/management"
	grpcomni "github.com/siderolabs/omni/internal/backend/grpc"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/audit/auditlog"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/audit/auditlog/auditlogsqlite"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/sqlite"
	"github.com/siderolabs/omni/internal/pkg/config"
)

// TestFollowAuditLogEndToEnd exercises the whole follow stack together, the real SQLite
// store behind the real handler served over a real gRPC connection to the real client
// library, across the situations a long-running follower meets: lease expiries, the log
// being wiped by cleanup, and the server going away mid-stream.
func TestFollowAuditLogEndToEnd(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
	t.Cleanup(cancel)

	store := newE2EStore(ctx, t)

	var (
		listener    atomic.Pointer[bufconn.Listener]
		numStreams  atomic.Int64
		grpcServers []*grpc.Server
	)

	startServer := func() *grpc.Server {
		server := grpcomni.NewManagementServer(nil, nil, zaptest.NewLogger(t), false, nil, nil,
			grpcomni.WithAuditLogger(&e2eAuditor{store}),
			grpcomni.WithAuditLogFollowLease(e2eLease))

		grpcServer := grpc.NewServer(grpc.StreamInterceptor(
			func(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
				numStreams.Add(1)

				return handler(srv, &authStream{
					ServerStream: ss,
					ctx:          managementPowerTestContext(ss.Context(), "admin@example.com", role.Admin),
				})
			},
		))

		management.RegisterManagementServiceServer(grpcServer, server)

		lis := bufconn.Listen(1024 * 1024)
		listener.Store(lis)

		go grpcServer.Serve(lis) //nolint:errcheck

		grpcServers = append(grpcServers, grpcServer)

		return grpcServer
	}

	firstServer := startServer()

	t.Cleanup(func() {
		for _, srv := range grpcServers {
			srv.Stop()
		}
	})

	conn, err := grpc.NewClient("passthrough:///audit-log-e2e",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(dialCtx context.Context, _ string) (net.Conn, error) {
			return listener.Load().DialContext(dialCtx)
		}))
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, conn.Close())
	})

	cli := managementcli.NewClient(conn)

	// phase 1: the backlog is delivered, and events written while the stream keeps cycling
	// through lease expiries and transparent reconnects keep arriving
	writeE2EEvents(ctx, t, store, 1, 5)

	follower := startFollower(ctx, t, cli, &management.ReadAuditLogRequest{StartTsMs: 1})

	follower.requireEvents(t, idRange(1, 5))

	require.Eventually(t, func() bool {
		return numStreams.Load() >= 2
	}, 10*time.Second, 10*time.Millisecond, "at least one lease expiry must force a reconnect")

	writeE2EEvents(ctx, t, store, 6, 10)
	follower.requireEvents(t, idRange(6, 10))

	// phase 2: cleanup covering every event spares the newest one, so the storage ids keep
	// increasing and the follower keeps receiving events without a reconnect
	require.NoError(t, store.Remove(ctx, time.UnixMilli(0), time.UnixMilli(baseE2ETsMs+10)))

	writeE2EEvents(ctx, t, store, 11, 11)
	follower.requireEvents(t, idRange(11, 11))

	// phase 3: the server goes away mid-stream, the iterator surfaces the error to the
	// caller, and re-invoking with the id of the last received event plus one resumes
	// exactly, the way the exporter retries after a failure
	firstServer.Stop()

	streamErr := follower.requireError(t)
	require.NotErrorIs(t, streamErr, managementcli.ErrAuditLogFollowUnsupported)

	startServer()

	writeE2EEvents(ctx, t, store, 12, 12)

	resumed := startFollower(ctx, t, cli, &management.ReadAuditLogRequest{FromId: 12})
	resumed.requireEvents(t, idRange(12, 12))
}

// TestFollowAuditLogWakesOnWrite proves the write notification over the whole stack. The
// lease is minutes away, so nothing but the wakeup can deliver a live event before the
// receive helper gives up: with a broken notification this test fails, while the lease
// reconnects of [TestFollowAuditLogEndToEnd] would mask it there by rediscovering the
// events on every fresh stream.
func TestFollowAuditLogWakesOnWrite(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
	t.Cleanup(cancel)

	store := newE2EStore(ctx, t)

	server := grpcomni.NewManagementServer(nil, nil, zaptest.NewLogger(t), false, nil, nil,
		grpcomni.WithAuditLogger(&e2eAuditor{store}),
		grpcomni.WithAuditLogFollowLease(10*time.Minute))

	grpcServer := grpc.NewServer(grpc.StreamInterceptor(
		func(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
			return handler(srv, &authStream{
				ServerStream: ss,
				ctx:          managementPowerTestContext(ss.Context(), "admin@example.com", role.Admin),
			})
		},
	))

	management.RegisterManagementServiceServer(grpcServer, server)

	lis := bufconn.Listen(1024 * 1024)

	go grpcServer.Serve(lis) //nolint:errcheck

	t.Cleanup(grpcServer.Stop)

	conn, err := grpc.NewClient("passthrough:///audit-log-notify",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(dialCtx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(dialCtx)
		}))
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, conn.Close())
	})

	follower := startFollower(ctx, t, managementcli.NewClient(conn), &management.ReadAuditLogRequest{StartTsMs: 1})

	// the first event can race the initial scan of the stream, so receiving it proves
	// nothing yet: it pins the stream past its backlog, parked waiting for a wakeup
	writeE2EEvents(ctx, t, store, 1, 1)
	follower.requireEvents(t, idRange(1, 1))

	// this is the load-bearing write: the stream is parked, and only the wakeup can
	// deliver the event before the receive helper times out
	writeE2EEvents(ctx, t, store, 2, 2)
	follower.requireEvents(t, idRange(2, 2))
}

// baseE2ETsMs keeps the synthetic event timestamps positive and distinct.
const baseE2ETsMs = 1_000_000

// e2eLease is long enough for the wipe-recovery timing assertion to tell lost-position
// detection apart from a lease expiry, and short enough to cycle a few times in the test.
const e2eLease = 3 * time.Second

// idRange returns the expected event ids: the events are written sequentially, so their ids
// match their ordinals, and cleanup never makes the ids restart.
func idRange(from, to int64) []int64 {
	result := make([]int64, 0, to-from+1)

	for i := from; i <= to; i++ {
		result = append(result, i)
	}

	return result
}

func writeE2EEvents(ctx context.Context, t *testing.T, store *auditlogsqlite.Store, from, to int64) {
	t.Helper()

	for i := from; i <= to; i++ {
		require.NoError(t, store.Write(ctx, auditlog.Event{
			Type:       "create",
			TimeMillis: baseE2ETsMs + i,
		}))
	}
}

func newE2EStore(ctx context.Context, t *testing.T) *auditlogsqlite.Store {
	t.Helper()

	conf := config.Default().Storage.Sqlite
	conf.SetPath(filepath.Join(t.TempDir(), "audit.db"))

	db, err := sqlite.OpenDB(conf)
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, db.Close())
	})

	store, err := auditlogsqlite.NewStore(ctx, db, 30*time.Second, 0, 0, zaptest.NewLogger(t))
	require.NoError(t, err)

	return store
}

// e2eAuditor serves follow reads from a real store, with the access-event methods stubbed out
// so that the followed stream carries only the events the test writes.
type e2eAuditor struct {
	*auditlogsqlite.Store
}

func (a *e2eAuditor) AuditTalosAccess(context.Context, string, string, string) error { return nil }

func (a *e2eAuditor) AuditAuditLogAccess(context.Context, auditlog.ReadFilters) error { return nil }

func (a *e2eAuditor) AuditAuditLogFollow(context.Context, int64, int64) error { return nil }

// authStream overrides the stream context with one carrying admin authentication, standing in
// for the signature interceptor of the full server.
type authStream struct {
	grpc.ServerStream

	ctx context.Context //nolint:containedctx
}

func (s *authStream) Context() context.Context {
	return s.ctx
}

// e2eFollower consumes a follow iterator on a goroutine, handing out the received event ids
// and the terminal error.
type e2eFollower struct {
	idCh  chan int64
	errCh chan error
}

func startFollower(ctx context.Context, t *testing.T, cli *managementcli.Client, req *management.ReadAuditLogRequest) *e2eFollower {
	t.Helper()

	follower := &e2eFollower{
		idCh:  make(chan int64, 1024),
		errCh: make(chan error, 1),
	}

	followCtx, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)

	go func() {
		for resp, err := range cli.FollowAuditLog(followCtx, req) {
			if err != nil {
				follower.errCh <- err

				return
			}

			// skip the yielded acknowledgments, the test tracks the events
			if len(resp.AuditLog) == 0 {
				continue
			}

			follower.idCh <- resp.Id
		}
	}()

	return follower
}

// requireEvents waits until every expected event id has been received, in order, asserting
// the exact-resume contract: no duplicates and nothing unexpected, even across reconnects.
func (f *e2eFollower) requireEvents(t *testing.T, expected []int64) {
	t.Helper()

	for _, want := range expected {
		select {
		case id := <-f.idCh:
			require.Equal(t, want, id, "events must arrive exactly once, in order")
		case err := <-f.errCh:
			require.NoError(t, err, "the stream failed while waiting for event %d", want)
		case <-time.After(30 * time.Second):
			require.Failf(t, "timed out", "waiting for event %d", want)
		}
	}
}

// requireError waits for the follow iterator to surface a terminal error.
func (f *e2eFollower) requireError(t *testing.T) error {
	t.Helper()

	select {
	case err := <-f.errCh:
		require.Error(t, err)

		return err
	case <-time.After(30 * time.Second):
		require.Fail(t, "timed out waiting for the stream error")

		return nil
	}
}
