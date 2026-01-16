// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package audit_test

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/siderolabs/gen/xtesting/must"
	"github.com/siderolabs/go-pointer"
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

//go:embed auditlog/auditlogfile/testdata/log/2012-01-01.jsonlog
var logData string

func TestAudit(t *testing.T) {
	config := config.LogsAudit{
		Enabled: pointer.To(true),
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
		ad := makeAuditData("Mozilla/5.0", "10.10.0.1", "")

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

	rdr, err := l.Reader(t.Context(), time.Time{}, time.Now().Add(5*time.Second))
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

func makeAuditData(agent, _, email string) auditlog.Data {
	return auditlog.Data{
		Session: auditlog.Session{
			UserAgent: agent,
			Email:     email,
		},
	}
}

func testDB(t *testing.T) *sql.DB {
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
