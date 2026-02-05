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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"zombiezen.com/go/sqlite/sqlitex"

	"github.com/siderolabs/omni/client/pkg/omni/resources/common"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/audit"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/audit/hooks"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/sqlite"
	"github.com/siderolabs/omni/internal/pkg/config"
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

func testDB(t *testing.T) *sqlitex.Pool {
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
