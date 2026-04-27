// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package audit_test

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/state-sqlite/pkg/sqlitexx"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/siderolabs/gen/xtesting/must"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/audit"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/audit/auditlog"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/audit/hooks"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/sqlite"
	"github.com/siderolabs/omni/internal/pkg/config"
	"github.com/siderolabs/omni/internal/pkg/ctxstore"
)

//go:embed testdata/expected_events.jsonlog
var logData string

func TestAudit(t *testing.T) {
	config := config.LogsAudit{
		Enabled: new(true),
	}

	db := testDB(t)

	l := must.Value(audit.NewLog(t.Context(), config, db, zaptest.NewLogger(t)))(t)

	hooks.Init(l)

	res := auth.NewPublicKey("917e47635eb900d0ae66271dd1e06966e048c4f3")

	res.Metadata().Labels().Set(auth.LabelPublicKeyUserID, "002cf196-1767-43fd-8e3d-91241e2ce70c")

	res.TypedSpec().Value.Identity = &specs.Identity{Email: "dmitry.matrenichev@siderolabs.com"}
	res.TypedSpec().Value.Role = "Admin"
	res.TypedSpec().Value.PublicKey = nil
	res.TypedSpec().Value.Expiration = timestamppb.New(time.Unix(1325587579, 0))

	createCtx := func() context.Context {
		ad := auditlog.Data{
			Session: auditlog.Session{
				UserAgent: "Mozilla/5.0",
			},
		}

		return ctxstore.WithValue(t.Context(), &ad)
	}

	actions := []func(*testing.T){
		func(t *testing.T) {
			fn := l.LogCreate(res)

			require.NoError(t, fn(createCtx(), res))
		},
		func(t *testing.T) {
			newRes := res.DeepCopy().(*auth.PublicKey) //nolint:errcheck,forcetypeassert
			newRes.TypedSpec().Value.Confirmed = true
			fn := l.LogUpdate(res)

			require.NoError(t, fn(createCtx(), res, newRes))

			res = newRes
		},
		func(t *testing.T) {
			newRes := res.DeepCopy().(*auth.PublicKey) //nolint:errcheck,forcetypeassert
			newRes.TypedSpec().Value.Confirmed = false
			fn := l.LogUpdateWithConflicts(res.Metadata())

			require.NoError(t, fn(createCtx(), res, newRes))

			res = newRes
		},
		func(t *testing.T) {
			fn := l.LogDestroy(res.Metadata())

			require.NoError(t, fn(createCtx(), res.Metadata()))
		},
		func(t *testing.T) {
			fn := l.LogCreate(res)

			require.NoError(t, fn(createCtx(), res))
		},
	}

	for _, action := range actions {
		action(t)
	}

	rdr, err := l.Reader(t.Context(), auditlog.ReadFilters{End: time.Now().Add(5 * time.Second)})
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, rdr.Close())
	})

	var sb strings.Builder

	for {
		data, err := rdr.Read()
		if errors.Is(err, io.EOF) {
			break
		}

		require.NoError(t, err)

		sb.Write(data)
	}

	cmpIgnoreTime(t, logData, sb.String())
}

func TestPhaseChangeIsAudited(t *testing.T) {
	config := config.LogsAudit{
		Enabled: new(true),
	}

	db := testDB(t)

	l := must.Value(audit.NewLog(t.Context(), config, db, zaptest.NewLogger(t)))(t)

	hooks.Init(l)

	res := auth.NewPublicKey("phase-change-test")

	res.Metadata().Labels().Set(auth.LabelPublicKeyUserID, "002cf196-1767-43fd-8e3d-91241e2ce70c")

	res.TypedSpec().Value.Identity = &specs.Identity{Email: "test@siderolabs.com"}
	res.TypedSpec().Value.Role = "Admin"
	res.TypedSpec().Value.Expiration = timestamppb.New(time.Unix(1325587579, 0))

	createCtx := func() context.Context {
		ad := auditlog.Data{
			Session: auditlog.Session{
				UserAgent: "Mozilla/5.0",
			},
		}

		return ctxstore.WithValue(t.Context(), &ad)
	}

	// First, create the resource.
	fn := l.LogCreate(res)
	require.NoError(t, fn(createCtx(), res))

	// Now update with only the phase changed (same spec).
	newRes := res.DeepCopy().(*auth.PublicKey) //nolint:errcheck,forcetypeassert
	newRes.Metadata().SetPhase(resource.PhaseTearingDown)

	updateFn := l.LogUpdate(res)
	require.NoError(t, updateFn(createCtx(), res, newRes))

	// Read all events and verify we got a teardown event.
	rdr, err := l.Reader(t.Context(), auditlog.ReadFilters{End: time.Now().Add(5 * time.Second)})
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, rdr.Close())
	})

	var events []map[string]any

	for {
		data, err := rdr.Read()
		if errors.Is(err, io.EOF) {
			break
		}

		require.NoError(t, err)

		var event map[string]any

		require.NoError(t, json.Unmarshal(data, &event))

		events = append(events, event)
	}

	require.Len(t, events, 2, "expected create + teardown events")
	require.Equal(t, "create", events[0]["event_type"])
	require.Equal(t, "teardown", events[1]["event_type"])
}

func TestK8SAccessAuditSkipsReadLikeRequests(t *testing.T) {
	testCases := []struct {
		name        string
		method      string
		target      string
		expectAudit bool
	}{
		{
			name:   "get",
			method: http.MethodGet,
			target: "/api/v1/namespaces/default/pods",
		},
		{
			name:   "head",
			method: http.MethodHead,
			target: "/api/v1/namespaces/default/pods",
		},
		{
			name:   "options",
			method: http.MethodOptions,
			target: "/api/v1/namespaces/default/pods",
		},
		{
			name:   "server side dry run patch",
			method: http.MethodPatch,
			target: "/apis/apps/v1/namespaces/default/deployments/guestbook-ui?dryRun=All&fieldManager=argocd-controller",
		},
		{
			name:   "server side dry run delete",
			method: http.MethodDelete,
			target: "/api/v1/namespaces/default/configmaps/test?dryRun=All",
		},
		{
			name:        "invalid bare dry run remains audited",
			method:      http.MethodPatch,
			target:      "/apis/apps/v1/namespaces/default/deployments/guestbook-ui?dryRun",
			expectAudit: true,
		},
		{
			name:        "invalid false dry run remains audited",
			method:      http.MethodPatch,
			target:      "/apis/apps/v1/namespaces/default/deployments/guestbook-ui?dryRun=false",
			expectAudit: true,
		},
		{
			name:   "self subject access review",
			method: http.MethodPost,
			target: "/apis/authorization.k8s.io/v1/selfsubjectaccessreviews",
		},
		{
			name:        "subject access review remains audited",
			method:      http.MethodPost,
			target:      "/apis/authorization.k8s.io/v1/subjectaccessreviews",
			expectAudit: true,
		},
		{
			name:        "create remains audited",
			method:      http.MethodPost,
			target:      "/api/v1/namespaces",
			expectAudit: true,
		},
		{
			name:        "patch remains audited",
			method:      http.MethodPatch,
			target:      "/apis/apps/v1/namespaces/default/deployments/guestbook-ui",
			expectAudit: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := config.LogsAudit{
				Enabled: new(true),
			}

			l := must.Value(audit.NewLog(t.Context(), config, testDB(t), zaptest.NewLogger(t)))(t)
			handler := l.Wrap(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			}))

			ctx := ctxstore.WithValue(t.Context(), &auditlog.Data{
				K8SAccess: &auditlog.K8SAccess{
					FullMethodName: tc.method + " " + strings.Split(tc.target, "?")[0],
					ClusterName:    "cluster1",
				},
				Session: auditlog.Session{
					Email: "user@example.com",
				},
			})
			req := httptest.NewRequestWithContext(ctx, tc.method, tc.target, nil)

			handler.ServeHTTP(httptest.NewRecorder(), req)

			events := readAuditEvents(t, l)
			if !tc.expectAudit {
				require.Empty(t, events)

				return
			}

			require.Len(t, events, 1)
			require.Equal(t, "k8s_access", events[0]["event_type"])
		})
	}
}

func readAuditEvents(t *testing.T, l *audit.Log) []map[string]any {
	t.Helper()

	rdr, err := l.Reader(t.Context(), auditlog.ReadFilters{End: time.Now().Add(5 * time.Second)})
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, rdr.Close())
	})

	var events []map[string]any

	for {
		data, err := rdr.Read()
		if errors.Is(err, io.EOF) {
			break
		}

		require.NoError(t, err)

		var event map[string]any

		require.NoError(t, json.Unmarshal(data, &event))

		events = append(events, event)
	}

	return events
}

func cmpIgnoreTime(t *testing.T, expected string, actual string) {
	expectedEvents := loadEvents(t, expected)
	actualEvents := loadEvents(t, actual)

	diff := cmp.Diff(expectedEvents, actualEvents, cmpopts.IgnoreMapEntries(func(k string, v any) bool {
		_, ok := v.(json.Number)

		return ok && k == "event_ts"
	}))
	if diff != "" {
		t.Fatalf("events mismatch (-want +got):\n%s", diff)
	}
}

func loadEvents(t *testing.T, events string) []any {
	var result []any

	decoder := json.NewDecoder(strings.NewReader(events))
	decoder.UseNumber()

	for {
		var event any

		err := decoder.Decode(&event)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			t.Fatalf("failed to decode event: %v", err)
		}

		result = append(result, event)
	}

	return result
}

func testDB(t *testing.T) *sqlitexx.Pool {
	t.Helper()

	conf := config.Default().Storage.Sqlite
	conf.SetPath(filepath.Join(t.TempDir(), "test.db"))

	db, err := sqlite.OpenDB(conf)
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, db.Close())
	})

	return db
}
