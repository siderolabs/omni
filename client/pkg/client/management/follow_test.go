// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package management_test

import (
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/omni/client/api/omni/management"
	managementcli "github.com/siderolabs/omni/client/pkg/client/management"
)

// scriptedStream scripts one follow stream: the responses it serves, and the error it ends
// with afterwards (nil is a clean end, the way the server ends a stream at its lifetime bound).
type scriptedStream struct {
	err       error
	responses []*management.ReadAuditLogResponse
}

// ack builds the event-less acknowledgment response carrying the resolved start position.
func ack(pos int64) *management.ReadAuditLogResponse {
	return &management.ReadAuditLogResponse{Id: pos}
}

func event(id int64, payload string) *management.ReadAuditLogResponse {
	return &management.ReadAuditLogResponse{AuditLog: []byte(payload + "\n"), Id: id}
}

// startPosition is the follow position of one connection attempt: the id to resume from, and
// the timestamp field it fell back to when no id was known yet.
type startPosition struct {
	fromID    int64
	startTsMs int64
}

// received is one response yielded by the follow iterator: the acknowledgments carry an empty
// payload, the events a newline-terminated one.
type received struct {
	payload string
	id      int64
}

func ackReceived(pos int64) received {
	return received{id: pos}
}

func eventReceived(id int64, payload string) received {
	return received{id: id, payload: payload + "\n"}
}

// fakeServiceClient serves one scripted stream per connection attempt and records the start
// position and the context of each attempt.
type fakeServiceClient struct {
	management.ManagementServiceClient

	t           *testing.T
	streams     []scriptedStream
	requests    []startPosition
	requestCtxs []context.Context
}

func (c *fakeServiceClient) ReadAuditLog(
	ctx context.Context, req *management.ReadAuditLogRequest, _ ...grpc.CallOption,
) (grpc.ServerStreamingClient[management.ReadAuditLogResponse], error) {
	require.True(c.t, req.Follow, "every request must be a follow request")
	require.Less(c.t, len(c.requests), len(c.streams), "more connection attempts than scripted streams")

	c.requests = append(c.requests, startPosition{fromID: req.FromId, startTsMs: req.StartTsMs})
	c.requestCtxs = append(c.requestCtxs, ctx)

	stream := c.streams[len(c.requests)-1]

	return &fakeStream{responses: stream.responses, err: stream.err}, nil
}

type fakeStream struct {
	grpc.ClientStream

	err       error
	responses []*management.ReadAuditLogResponse
}

func (s *fakeStream) Recv() (*management.ReadAuditLogResponse, error) {
	if len(s.responses) == 0 {
		if s.err != nil {
			return nil, s.err
		}

		return nil, io.EOF
	}

	resp := s.responses[0]
	s.responses = s.responses[1:]

	return resp, nil
}

// collectFollow drains the follow iterator, returning everything it yields, acknowledgments
// included, along with the terminal error.
func collectFollow(ctx context.Context, t *testing.T, cli *fakeServiceClient, req *management.ReadAuditLogRequest) ([]received, error) {
	t.Helper()

	var results []received

	for resp, err := range managementcli.NewTestClient(cli).FollowAuditLog(ctx, req) {
		if err != nil {
			return results, err
		}

		results = append(results, received{id: resp.Id, payload: string(resp.AuditLog)})
	}

	return results, nil
}

var errStream = status.Error(codes.Unavailable, "stream failure")

func TestFollowAuditLogReconnectsOnCleanEnd(t *testing.T) {
	cli := &fakeServiceClient{
		t: t,
		streams: []scriptedStream{
			// backlog, then the server ends the stream at its lifetime bound
			{responses: []*management.ReadAuditLogResponse{ack(0), event(1, "a"), event(2, "b")}},
			// the resume is exact: nothing is repeated, nothing is skipped
			{responses: []*management.ReadAuditLogResponse{ack(2), event(3, "c")}},
			{err: errStream},
		},
	}

	results, err := collectFollow(t.Context(), t, cli, &management.ReadAuditLogRequest{})
	require.ErrorIs(t, err, errStream, "a non-clean error terminates the iteration")

	assert.Equal(t, []received{
		ackReceived(0), eventReceived(1, "a"), eventReceived(2, "b"),
		ackReceived(2), eventReceived(3, "c"),
	}, results, "every event exactly once, with the acknowledgments yielded as position-only responses")
	assert.Equal(t, []startPosition{{}, {fromID: 3}, {fromID: 4}}, cli.requests,
		"each reconnect resumes exactly after the last delivered event")
}

func TestFollowAuditLogAckEstablishesRecoveryBaseline(t *testing.T) {
	cli := &fakeServiceClient{
		t: t,
		streams: []scriptedStream{
			// the stream fails right after the acknowledgment, before any event: the yielded
			// acknowledgment is what lets the consumer persist the resolved position and
			// resume exactly, e.g. picking up an event committed during the failure
			{responses: []*management.ReadAuditLogResponse{ack(40)}, err: errStream},
		},
	}

	results, err := collectFollow(t.Context(), t, cli, &management.ReadAuditLogRequest{})
	require.ErrorIs(t, err, errStream)

	assert.Equal(t, []received{ackReceived(40)}, results,
		"the consumer observes the resolved position even when no event was delivered")
}

func TestFollowAuditLogAckOnlyLeaseResumesFromAck(t *testing.T) {
	cli := &fakeServiceClient{
		t: t,
		streams: []scriptedStream{
			// the stream ends cleanly before any event: the position resolved by the
			// acknowledgment is the resume baseline, so nothing written during the gap is skipped
			{responses: []*management.ReadAuditLogResponse{ack(7)}},
			{responses: []*management.ReadAuditLogResponse{ack(7), event(8, "a")}},
			{err: errStream},
		},
	}

	results, err := collectFollow(t.Context(), t, cli, &management.ReadAuditLogRequest{})
	require.ErrorIs(t, err, errStream)

	assert.Equal(t, []received{ackReceived(7), ackReceived(7), eventReceived(8, "a")}, results)
	assert.Equal(t, []startPosition{{}, {fromID: 8}, {fromID: 9}}, cli.requests)
}

func TestFollowAuditLogTimestampStartBecomesIDResume(t *testing.T) {
	cli := &fakeServiceClient{
		t: t,
		streams: []scriptedStream{
			// the first attempt positions by timestamp, and the acknowledgment converts the
			// position into an id: the timestamp is out of the picture from then on
			{responses: []*management.ReadAuditLogResponse{ack(41)}},
			{responses: []*management.ReadAuditLogResponse{ack(41), event(42, "a")}},
			{err: errStream},
		},
	}

	results, err := collectFollow(t.Context(), t, cli, &management.ReadAuditLogRequest{StartTsMs: 7500})
	require.ErrorIs(t, err, errStream)

	assert.Equal(t, []received{ackReceived(41), ackReceived(41), eventReceived(42, "a")}, results)
	assert.Equal(t, []startPosition{{startTsMs: 7500}, {fromID: 42}, {fromID: 43}}, cli.requests,
		"the timestamp is only sent before the first acknowledgment")
}

func TestFollowAuditLogRejectsOldServer(t *testing.T) {
	for name, streams := range map[string][]scriptedStream{
		// an old server ignores the follow flag and serves a bounded read: events with no
		// acknowledgment first, or, on an empty backlog, a clean end with no message at all
		"event without ack":             {{responses: []*management.ReadAuditLogResponse{event(1, "a")}}},
		"clean end without any message": {{}},
	} {
		t.Run(name, func(t *testing.T) {
			cli := &fakeServiceClient{t: t, streams: streams}

			results, err := collectFollow(t.Context(), t, cli, &management.ReadAuditLogRequest{})
			require.ErrorIs(t, err, managementcli.ErrAuditLogFollowUnsupported)
			assert.Empty(t, results, "nothing must be yielded on an unsupported server")
		})
	}
}

func TestFollowAuditLogRejectsMalformedEvent(t *testing.T) {
	cli := &fakeServiceClient{
		t: t,
		streams: []scriptedStream{
			// an event without an id has no usable resume position
			{responses: []*management.ReadAuditLogResponse{ack(0), event(1, "a"), {AuditLog: []byte("x\n")}}},
		},
	}

	results, err := collectFollow(t.Context(), t, cli, &management.ReadAuditLogRequest{})
	require.ErrorContains(t, err, "malformed")
	assert.Equal(t, []received{ackReceived(0), eventReceived(1, "a")}, results)
}

func TestFollowAuditLogEarlyStopTearsDownStream(t *testing.T) {
	cli := &fakeServiceClient{
		t: t,
		streams: []scriptedStream{
			{responses: []*management.ReadAuditLogResponse{ack(0), event(1, "a"), event(2, "b")}},
		},
	}

	// the consumer stops after the first event: the stream context must be canceled, so the
	// abandoned stream does not linger on the server until its lease ends
	for resp, err := range managementcli.NewTestClient(cli).FollowAuditLog(t.Context(), &management.ReadAuditLogRequest{}) {
		require.NoError(t, err)

		if len(resp.AuditLog) != 0 {
			break
		}
	}

	require.Len(t, cli.requestCtxs, 1)
	require.ErrorIs(t, cli.requestCtxs[0].Err(), context.Canceled, "stopping the iteration must cancel the stream context")
}

func TestFollowAuditLogStopsOnCanceledContext(t *testing.T) {
	cli := &fakeServiceClient{
		t: t,
		streams: []scriptedStream{
			{responses: []*management.ReadAuditLogResponse{ack(0), event(1, "a")}},
		},
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	// the canceled context surfaces as the terminal error instead of reconnecting forever
	results, err := collectFollow(ctx, t, cli, &management.ReadAuditLogRequest{})
	require.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, []received{ackReceived(0), eventReceived(1, "a")}, results)
}
