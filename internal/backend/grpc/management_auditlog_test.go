// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package grpc_test

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

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

func newAuditLogTestServer(t *testing.T, auditor *auditLogAccessAuditor) *grpcomni.ManagementServer {
	t.Helper()

	return grpcomni.NewManagementServer(nil, nil, zaptest.NewLogger(t), false, nil, nil, grpcomni.WithAuditLogger(auditor))
}

type auditLogAccessAuditor struct {
	accessErr     error
	accessFilters *auditlog.ReadFilters
	events        [][]byte
	readerCalled  bool
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

type auditLogStream struct {
	grpc.ServerStream

	ctx  context.Context //nolint:containedctx
	sent [][]byte
}

func (s *auditLogStream) Context() context.Context {
	return s.ctx
}

func (s *auditLogStream) Send(resp *management.ReadAuditLogResponse) error {
	s.sent = append(s.sent, resp.AuditLog)

	return nil
}
