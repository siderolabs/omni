// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package audit_test

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/siderolabs/gen/pair"
	"github.com/siderolabs/gen/xtesting/must"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/audit"
	"github.com/siderolabs/omni/internal/pkg/ctxstore"
)

//go:embed testdata/log
var logDir embed.FS

func TestLog(t *testing.T) {
	tempDir := t.TempDir()
	logger := must.Value(audit.NewLogger(tempDir, zaptest.NewLogger(t)))(t)

	logger.ShoudLog(audit.EventCreate|audit.EventUpdate|audit.EventUpdateWithConflicts,
		pair.MakePair(auth.PublicKeyType, audit.AllowAll),
	)

	events := []pair.Triple[audit.EventType, resource.Type, *audit.Data]{
		pair.MakeTriple(audit.EventCreate, auth.PublicKeyType, &audit.Data{UserAgent: "Mozilla/5.0", IPAddress: "10.10.0.1", Email: "random_email1@example.com"}),
		pair.MakeTriple(audit.EventUpdate, auth.PublicKeyType, &audit.Data{UserAgent: "Mozilla/5.0", IPAddress: "10.10.0.2", Email: "random_email2@example.com"}),
		pair.MakeTriple(audit.EventUpdateWithConflicts, auth.PublicKeyType, &audit.Data{UserAgent: "Mozilla/5.0", IPAddress: "10.10.0.3", Email: "random_email3@example.com"}),
		pair.MakeTriple(audit.EventDestroy, auth.PublicKeyType, &audit.Data{UserAgent: "Mozilla/5.0", IPAddress: "10.10.0.4", Email: "random_email4@example.com"}),
		pair.MakeTriple(audit.EventCreate, auth.PublicKeyType, (*audit.Data)(nil)),
		pair.MakeTriple(audit.EventCreate, auth.AuthConfigType, &audit.Data{UserAgent: "Mozilla/5.0", IPAddress: "10.10.0.5", Email: "random_email5@example.com"}),
	}

	for _, event := range events {
		ctx := context.Background()

		if event.V3 != nil {
			ctx = ctxstore.WithValue(ctx, event.V3)
		}

		logger.LogEvent(ctx, event.V1, event.V2, 100)
	}

	equalDirs(
		t,
		&wrapFS{
			subFS: fsSub(t, logDir, "log"),
			File:  "2012-01-01.jsonlog",
		},
		os.DirFS(tempDir).(subFS), //nolint:forcetypeassert
		cmpIgnoreTime,
	)
}

type wrapFS struct {
	subFS
	File string
}

func (w *wrapFS) ReadFile(string) ([]byte, error) {
	return w.subFS.ReadFile(w.File)
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

func loadEvents(t *testing.T, expected string) []any {
	var result []any

	decoder := json.NewDecoder(strings.NewReader(expected))
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
