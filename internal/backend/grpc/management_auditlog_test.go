// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package grpc_test

import (
	"context"
	"errors"
	"io"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/omni/client/api/omni/management"
	"github.com/siderolabs/omni/client/pkg/access/role"
	grpcomni "github.com/siderolabs/omni/internal/backend/grpc"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/audit/auditlog"
)

func TestReadAuditLogAuditsAccess(t *testing.T) {
	auditor := &auditLogAccessAuditor{events: [][]byte{[]byte(`{"event_type":"create"}`)}}
	server := newAuditLogTestServer(t, auditor)

	stream := &auditLogStream{ctx: managementPowerTestContext(t.Context(), "admin@example.com", role.Admin)}

	err := server.ReadAuditLog(&management.ReadAuditLogRequest{
		StartTime: "2026-01-01",
		EndTime:   "2026-01-31",
		Search:    "some-search",
		ClusterId: "cluster-1",
	}, stream)
	require.NoError(t, err)

	require.NotNil(t, auditor.accessFilters)
	require.Equal(t, time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local), auditor.accessFilters.Start) //nolint:gosmopolitan
	require.Equal(t, time.Date(2026, 1, 31, 0, 0, 0, 0, time.Local), auditor.accessFilters.End)  //nolint:gosmopolitan
	require.Equal(t, "some-search", auditor.accessFilters.Search)
	require.Equal(t, "cluster-1", auditor.accessFilters.ClusterID)

	require.Equal(t, [][]byte{[]byte(`{"event_type":"create"}`)}, stream.sent)
}

func TestReadAuditLogFailsClosedOnAuditError(t *testing.T) {
	auditor := &auditLogAccessAuditor{accessErr: errors.New("audit write failure")}
	server := newAuditLogTestServer(t, auditor)

	stream := &auditLogStream{ctx: managementPowerTestContext(t.Context(), "admin@example.com", role.Admin)}

	err := server.ReadAuditLog(&management.ReadAuditLogRequest{}, stream)
	require.ErrorContains(t, err, "audit write failure")

	require.False(t, auditor.readerCalled, "reader must not be opened when the access audit write fails")
	require.Empty(t, stream.sent)
}

func TestReadAuditLogRequiresAdmin(t *testing.T) {
	auditor := &auditLogAccessAuditor{}
	server := newAuditLogTestServer(t, auditor)

	stream := &auditLogStream{ctx: managementPowerTestContext(t.Context(), "user@example.com", role.Operator)}

	err := server.ReadAuditLog(&management.ReadAuditLogRequest{}, stream)
	require.Equal(t, codes.PermissionDenied, status.Code(err))

	require.Nil(t, auditor.accessFilters)
	require.False(t, auditor.readerCalled)
}

func TestFollowAuditLogRejectsIncompatibleFields(t *testing.T) {
	for _, testCase := range []struct {
		req  *management.ReadAuditLogRequest
		name string
	}{
		{name: "start_time", req: &management.ReadAuditLogRequest{StartTime: "2026-01-01"}},
		{name: "end_time", req: &management.ReadAuditLogRequest{EndTime: "2026-01-31"}},
		{name: "order_by_field", req: &management.ReadAuditLogRequest{OrderByField: management.AuditLogOrderByField_AUDIT_LOG_ORDER_BY_FIELD_ACTOR}},
		{name: "order_by_dir", req: &management.ReadAuditLogRequest{OrderByDir: management.AuditLogOrderByDir_AUDIT_LOG_ORDER_BY_DIR_DESC}},
		{name: "search", req: &management.ReadAuditLogRequest{Search: "something"}},
		{name: "event_type", req: &management.ReadAuditLogRequest{EventType: management.AuditLogEventType_AUDIT_LOG_EVENT_TYPE_CREATE}},
		{name: "resource_type", req: &management.ReadAuditLogRequest{ResourceType: "some.resource"}},
		{name: "resource_id", req: &management.ReadAuditLogRequest{ResourceId: "some-id"}},
		{name: "cluster_id", req: &management.ReadAuditLogRequest{ClusterId: "some-cluster"}},
		{name: "actor", req: &management.ReadAuditLogRequest{Actor: "someone@example.com"}},
		{name: "start_ts_ms", req: &management.ReadAuditLogRequest{StartTsMs: -1}},
		{name: "from_id", req: &management.ReadAuditLogRequest{FromId: -1}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			auditor := &auditLogAccessAuditor{}
			server := newAuditLogTestServer(t, auditor)
			stream := &auditLogStream{ctx: managementPowerTestContext(t.Context(), "admin@example.com", role.Admin)}

			testCase.req.Follow = true

			err := server.ReadAuditLog(testCase.req, stream)
			require.Equal(t, codes.InvalidArgument, status.Code(err))
			require.ErrorContains(t, err, testCase.name)

			require.False(t, auditor.followAudited)
			require.Empty(t, stream.numResponses())
		})
	}
}

func TestFollowAuditLogRequiresAdmin(t *testing.T) {
	auditor := &auditLogAccessAuditor{}
	server := newAuditLogTestServer(t, auditor)

	stream := &auditLogStream{ctx: managementPowerTestContext(t.Context(), "user@example.com", role.Operator)}

	err := server.ReadAuditLog(&management.ReadAuditLogRequest{Follow: true}, stream)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	require.Empty(t, stream.numResponses())
}

func TestFollowAuditLogFailsClosedOnAuditError(t *testing.T) {
	auditor := &auditLogAccessAuditor{accessErr: errors.New("audit write failure")}
	server := newAuditLogTestServer(t, auditor)

	stream := &auditLogStream{ctx: managementPowerTestContext(t.Context(), "admin@example.com", role.Admin)}

	err := server.ReadAuditLog(&management.ReadAuditLogRequest{Follow: true}, stream)
	require.ErrorContains(t, err, "audit write failure")

	require.Empty(t, stream.numResponses(), "nothing must be sent when the access audit write fails")
}

func TestFollowAuditLogAcknowledgesAndEndsOnLease(t *testing.T) {
	auditor := &auditLogAccessAuditor{}
	server := newAuditLogTestServer(t, auditor, grpcomni.WithAuditLogFollowLease(100*time.Millisecond))

	stream := &auditLogStream{ctx: managementPowerTestContext(t.Context(), "admin@example.com", role.Admin)}

	err := server.ReadAuditLog(&management.ReadAuditLogRequest{Follow: true, StartTsMs: 12345}, stream)
	require.NoError(t, err, "lease expiry must end the stream cleanly")

	require.True(t, auditor.followAudited)
	require.Equal(t, int64(12345), auditor.followStartTsMs)

	responses := stream.snapshotResponses()
	require.Len(t, responses, 1)
	require.Empty(t, responses[0].AuditLog, "the first response must be the event-less acknowledgment")
	require.Zero(t, responses[0].Id, "the acknowledgment carries the resolved start position")
}

func TestFollowAuditLogDeliversBacklogAndLiveEvents(t *testing.T) {
	auditor := &auditLogAccessAuditor{
		followStartPos: 1, // skip the first entry, as a follow resuming after it would
		followEntries: []auditlog.Entry{
			{ID: 1, Payload: []byte(`{"event_ts":100}` + "\n")},
			{ID: 2, Payload: []byte(`{"event_ts":200}` + "\n")},
			{ID: 3, Payload: []byte(`{"event_ts":300}` + "\n")},
		},
	}
	server := newAuditLogTestServer(t, auditor, grpcomni.WithAuditLogFollowLease(time.Minute))

	ctx, cancel := context.WithCancel(managementPowerTestContext(t.Context(), "admin@example.com", role.Admin))
	t.Cleanup(cancel)

	stream := &auditLogStream{ctx: ctx}

	errCh := make(chan error, 1)

	go func() {
		errCh <- server.ReadAuditLog(&management.ReadAuditLogRequest{Follow: true}, stream)
	}()

	// the acknowledgment and the backlog after the start position
	requireResponses(ctx, t, stream, 3)

	// an event written while the stream is live wakes it through the subscription
	auditor.appendFollowEntries(auditlog.Entry{ID: 4, Payload: []byte(`{"event_ts":400}` + "\n")})

	requireResponses(ctx, t, stream, 4)

	cancel()

	require.ErrorIs(t, <-errCh, context.Canceled)

	responses := stream.snapshotResponses()
	require.Empty(t, responses[0].AuditLog)

	payloads := make([]string, 0, len(responses)-1)
	ids := make([]int64, 0, len(responses)-1)

	for _, resp := range responses[1:] {
		payloads = append(payloads, string(resp.AuditLog))
		ids = append(ids, resp.Id)
	}

	require.Equal(t, []string{
		`{"event_ts":200}` + "\n",
		`{"event_ts":300}` + "\n",
		`{"event_ts":400}` + "\n",
	}, payloads)
	require.Equal(t, []int64{2, 3, 4}, ids)
}

func TestFollowAuditLogPaginatesBatches(t *testing.T) {
	// one entry beyond the server batch size, so the backlog takes a full batch plus one
	const numEntries = grpcomni.AuditLogFollowBatchSize + 1

	entries := make([]auditlog.Entry, 0, numEntries)
	for i := range numEntries {
		entries = append(entries, auditlog.Entry{ID: int64(i + 1), Payload: []byte("{}\n")})
	}

	auditor := &auditLogAccessAuditor{followEntries: entries}
	server := newAuditLogTestServer(t, auditor, grpcomni.WithAuditLogFollowLease(time.Minute))

	ctx, cancel := context.WithCancel(managementPowerTestContext(t.Context(), "admin@example.com", role.Admin))
	t.Cleanup(cancel)

	stream := &auditLogStream{ctx: ctx}

	errCh := make(chan error, 1)

	go func() {
		errCh <- server.ReadAuditLog(&management.ReadAuditLogRequest{Follow: true, StartTsMs: 1}, stream)
	}()

	requireResponses(ctx, t, stream, numEntries+1)

	cancel()

	require.ErrorIs(t, <-errCh, context.Canceled)

	responses := stream.snapshotResponses()
	for i, resp := range responses[1:] {
		require.Equal(t, int64(i+1), resp.Id)
	}
}

func TestFollowAuditLogFailsWithoutAckOnStartError(t *testing.T) {
	auditor := &auditLogAccessAuditor{followStartErr: errors.New("audit log is disabled")}
	server := newAuditLogTestServer(t, auditor, grpcomni.WithAuditLogFollowLease(time.Minute))

	stream := &auditLogStream{ctx: managementPowerTestContext(t.Context(), "admin@example.com", role.Admin)}

	// a stream that cannot resolve its start position fails before the acknowledgment,
	// so it never looks like a working follow to the client
	err := server.ReadAuditLog(&management.ReadAuditLogRequest{Follow: true}, stream)
	require.ErrorContains(t, err, "audit log is disabled")
	require.Empty(t, stream.numResponses())
}

func TestFollowAuditLogFailsOnLostPosition(t *testing.T) {
	auditor := &auditLogAccessAuditor{followBatchErr: auditlog.ErrFollowPositionLost}
	server := newAuditLogTestServer(t, auditor, grpcomni.WithAuditLogFollowLease(time.Minute))

	stream := &auditLogStream{ctx: managementPowerTestContext(t.Context(), "admin@example.com", role.Admin)}

	// a lost position means the database was replaced underneath the id the client resumes
	// from: there is no safe automatic recovery, so the stream fails for the operator
	err := server.ReadAuditLog(&management.ReadAuditLogRequest{Follow: true, FromId: 100}, stream)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}

func TestFollowAuditLogResumesFromID(t *testing.T) {
	auditor := &auditLogAccessAuditor{
		followEntries: []auditlog.Entry{
			{ID: 1, Payload: []byte(`{"event_ts":100}` + "\n")},
			{ID: 2, Payload: []byte(`{"event_ts":200}` + "\n")},
			{ID: 3, Payload: []byte(`{"event_ts":300}` + "\n")},
		},
	}
	server := newAuditLogTestServer(t, auditor, grpcomni.WithAuditLogFollowLease(200*time.Millisecond))

	stream := &auditLogStream{ctx: managementPowerTestContext(t.Context(), "admin@example.com", role.Admin)}

	// an id position resumes exactly: no timestamp resolution is involved
	err := server.ReadAuditLog(&management.ReadAuditLogRequest{Follow: true, FromId: 3}, stream)
	require.NoError(t, err)

	require.False(t, auditor.followStartCalled, "an id position must not be re-resolved")

	responses := stream.snapshotResponses()
	require.Len(t, responses, 2)
	require.Empty(t, responses[0].AuditLog)
	require.Equal(t, int64(2), responses[0].Id, "the acknowledgment carries the resolved position")
	require.Equal(t, int64(3), responses[1].Id)
}

func TestFollowAuditLogSendFailure(t *testing.T) {
	auditor := &auditLogAccessAuditor{}
	server := newAuditLogTestServer(t, auditor, grpcomni.WithAuditLogFollowLease(time.Minute))

	stream := &auditLogStream{
		ctx:     managementPowerTestContext(t.Context(), "admin@example.com", role.Admin),
		sendErr: errors.New("send failure"),
	}

	err := server.ReadAuditLog(&management.ReadAuditLogRequest{Follow: true}, stream)
	require.ErrorContains(t, err, "send failure")
}

// requireResponses waits until the stream has sent at least n responses.
func requireResponses(ctx context.Context, t *testing.T, stream *auditLogStream, n int) {
	t.Helper()

	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		assert.GreaterOrEqual(collect, stream.numResponses(), n)
	}, 10*time.Second, time.Millisecond, "expected %d responses, got %d (ctx err: %v)", n, stream.numResponses(), ctx.Err())
}

func newAuditLogTestServer(t *testing.T, auditor *auditLogAccessAuditor, opts ...grpcomni.ManagementServerOption) *grpcomni.ManagementServer {
	t.Helper()

	return grpcomni.NewManagementServer(nil, nil, zaptest.NewLogger(t), false, nil, nil,
		append([]grpcomni.ManagementServerOption{grpcomni.WithAuditLogger(auditor)}, opts...)...)
}

//nolint:govet // field grouping is preferred over alignment here
type auditLogAccessAuditor struct {
	accessErr         error
	followStartErr    error
	followBatchErr    error
	accessFilters     *auditlog.ReadFilters
	events            [][]byte
	followStartPos    int64
	followFromID      int64
	followStartTsMs   int64
	readerCalled      bool
	followAudited     bool
	followStartCalled bool

	mu            sync.Mutex
	followEntries []auditlog.Entry
	followWakeCh  chan struct{}
}

// appendFollowEntries adds live events to the fake store while a follow stream is running,
// waking the subscribed follower like the real store does after a write.
func (a *auditLogAccessAuditor) appendFollowEntries(entries ...auditlog.Entry) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.followEntries = append(a.followEntries, entries...)

	if a.followWakeCh != nil {
		select {
		case a.followWakeCh <- struct{}{}:
		default:
		}
	}
}

func (a *auditLogAccessAuditor) FollowSubscribe() (<-chan struct{}, func()) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.followWakeCh = make(chan struct{}, 1)

	return a.followWakeCh, func() {}
}

func (a *auditLogAccessAuditor) Reader(context.Context, auditlog.ReadFilters) (auditlog.Reader, error) {
	a.readerCalled = true

	return &sliceAuditLogReader{data: a.events}, nil
}

func (a *auditLogAccessAuditor) AuditTalosAccess(context.Context, string, string, string) error {
	return nil
}

func (a *auditLogAccessAuditor) AuditAuditLogAccess(_ context.Context, filters auditlog.ReadFilters) error {
	a.accessFilters = &filters

	return a.accessErr
}

func (a *auditLogAccessAuditor) AuditAuditLogFollow(_ context.Context, fromID, startTsMs int64) error {
	a.followAudited = true
	a.followFromID = fromID
	a.followStartTsMs = startTsMs

	return a.accessErr
}

func (a *auditLogAccessAuditor) FollowStart(context.Context, int64) (int64, error) {
	a.followStartCalled = true

	return a.followStartPos, a.followStartErr
}

func (a *auditLogAccessAuditor) FollowBatch(_ context.Context, afterID int64, limit int64) ([]auditlog.Entry, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.followBatchErr != nil {
		return nil, a.followBatchErr
	}

	var entries []auditlog.Entry

	for _, entry := range a.followEntries {
		if entry.ID > afterID && int64(len(entries)) < limit {
			entries = append(entries, entry)
		}
	}

	return entries, nil
}

type sliceAuditLogReader struct {
	data [][]byte
}

func (r *sliceAuditLogReader) Read() ([]byte, error) {
	if len(r.data) == 0 {
		return nil, io.EOF
	}

	data := r.data[0]
	r.data = r.data[1:]

	return data, nil
}

func (r *sliceAuditLogReader) Close() error {
	return nil
}

//nolint:govet // field grouping is preferred over alignment here
type auditLogStream struct {
	grpc.ServerStream

	ctx     context.Context //nolint:containedctx
	sent    [][]byte
	sendErr error

	mu        sync.Mutex
	responses []*management.ReadAuditLogResponse
}

func (s *auditLogStream) Context() context.Context {
	return s.ctx
}

func (s *auditLogStream) Send(resp *management.ReadAuditLogResponse) error {
	if s.sendErr != nil {
		return s.sendErr
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.sent = append(s.sent, resp.AuditLog)
	s.responses = append(s.responses, resp)

	return nil
}

// numResponses returns the number of responses sent so far.
func (s *auditLogStream) numResponses() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return len(s.responses)
}

// snapshotResponses returns a copy of the responses sent so far.
func (s *auditLogStream) snapshotResponses() []*management.ReadAuditLogResponse {
	s.mu.Lock()
	defer s.mu.Unlock()

	return slices.Clone(s.responses)
}
