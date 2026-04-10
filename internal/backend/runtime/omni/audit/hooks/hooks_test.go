// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package hooks_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/cosi-project/state-sqlite/pkg/sqlitexx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/siderolabs/omni/client/api/omni/specs"
	authres "github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/client/pkg/omni/resources/common"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/audit"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/audit/auditlog"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/audit/hooks"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/sqlite"
	"github.com/siderolabs/omni/internal/pkg/config"
	"github.com/siderolabs/omni/internal/pkg/ctxstore"
)

func TestUserManagedResourceTypes(t *testing.T) {
	t.Parallel()

	userManagedResourceTypes := common.UserManagedResourceTypes
	logger := zaptest.NewLogger(t)
	db := testDB(t)

	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()

	auditLog, err := audit.NewLog(ctx, config.Default().Logs.Audit, db, logger)
	assert.NoError(t, err)

	hooks.Init(auditLog)

	createHooksResourceTypes := auditLog.CreateHooksResourceTypes()
	updateHooksResourceTypes := auditLog.UpdateHooksResourceTypes()
	destroyHooksResourceTypes := auditLog.DestroyHooksResourceTypes()

	assert.Subset(t, createHooksResourceTypes, userManagedResourceTypes, "all user managed resource types should have create hooks")
	assert.Subset(t, updateHooksResourceTypes, userManagedResourceTypes, "all user managed resource types should have update hooks")
	assert.Subset(t, destroyHooksResourceTypes, userManagedResourceTypes, "all user managed resource types should have destroy hooks")
}

func TestPublicKeyCreatePreservesActorSession(t *testing.T) {
	t.Parallel()

	db := testDB(t)

	cfg := config.LogsAudit{Enabled: new(true)}
	l, err := audit.NewLog(t.Context(), cfg, db, zaptest.NewLogger(t))
	require.NoError(t, err)

	hooks.Init(l)

	saPublicKey := authres.NewPublicKey("sa-key-fingerprint")
	saPublicKey.Metadata().Labels().Set(authres.LabelPublicKeyUserID, "sa-user-id")
	saPublicKey.TypedSpec().Value.Identity = &specs.Identity{Email: "my-sa@serviceaccount.omni.sidero.dev"}
	saPublicKey.TypedSpec().Value.Role = "Operator"
	saPublicKey.TypedSpec().Value.Expiration = timestamppb.New(time.Unix(1700000000, 0))

	t.Run("admin managing service account preserves admin session", func(t *testing.T) {
		t.Parallel()

		ad := auditlog.Data{
			Session: auditlog.Session{
				UserAgent: "grpc-go/1.80.0",
				UserID:    "admin-user-id",
				Email:     "admin@siderolabs.com",
				Role:      "Admin",
			},
		}

		ctx := ctxstore.WithValue(t.Context(), &ad)

		fn := l.LogCreate(saPublicKey)
		require.NoError(t, fn(ctx, saPublicKey))

		assert.Equal(t, "admin@siderolabs.com", ad.Session.Email, "Session.Email must remain the admin's email")
		assert.Equal(t, "admin-user-id", ad.Session.UserID, "Session.UserID must remain the admin's user ID")
		assert.Equal(t, "Admin", string(ad.Session.Role), "Session.Role must remain the admin's role")
		assert.Equal(t, "sa-key-fingerprint", ad.Session.Fingerprint, "Session.Fingerprint should be set to the new public key ID")
	})

	t.Run("empty session gets populated from public key", func(t *testing.T) {
		t.Parallel()

		ad := auditlog.Data{
			Session: auditlog.Session{
				UserAgent: "Mozilla/5.0",
			},
		}

		ctx := ctxstore.WithValue(t.Context(), &ad)

		fn := l.LogCreate(saPublicKey)
		require.NoError(t, fn(ctx, saPublicKey))

		assert.Equal(t, "my-sa@serviceaccount.omni.sidero.dev", ad.Session.Email, "Session.Email should be set from the public key")
		assert.Equal(t, "sa-user-id", ad.Session.UserID, "Session.UserID should be set from the public key")
		assert.Equal(t, "Operator", string(ad.Session.Role), "Session.Role should be set from the public key")
		assert.Equal(t, "sa-key-fingerprint", ad.Session.Fingerprint, "Session.Fingerprint should be set to the public key ID")
	})
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
