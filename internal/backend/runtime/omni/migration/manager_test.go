// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package migration_test

import (
	"context"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/omni/client/pkg/omni/resources/system"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/migration"
)

func TestSkipDroppedMigrationsOnFreshInstall(t *testing.T) {
	st := state.WrapCore(namespaced.NewState(inmem.Build))
	logger := zaptest.NewLogger(t)

	mgr := migration.NewManager(st, logger)

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	result, err := mgr.Run(ctx)
	require.NoError(t, err)

	logger.Info("migration result", zap.Any("result", result))

	require.Equal(t, uint64(47), result.StartVersion, "expected start version to be the first migration after the dropped ones")
	require.Greater(t, result.DBVersion.TypedSpec().Value.Version, uint64(47))
}

func TestFailOnTooOldDBVersion(t *testing.T) {
	for _, tt := range []struct {
		name           string
		currentVersion uint64
		expectError    bool
	}{
		{"too old version", 35, true},
		{"just old enough version", 46, true},
		{"recent enough version", 47, false},
		{"more than enough version", 50, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			st := state.WrapCore(namespaced.NewState(inmem.Build))

			dbVersion := system.NewDBVersion(system.DBVersionID)
			dbVersion.TypedSpec().Value.Version = tt.currentVersion

			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()

			require.NoError(t, st.Create(ctx, dbVersion))

			logger := zaptest.NewLogger(t)
			mgr := migration.NewManager(st, logger)

			result, err := mgr.Run(ctx)
			if tt.expectError {
				require.ErrorIs(t, err, migration.ErrDropped)
			} else {
				require.NoError(t, err)
				require.Greater(t, result.DBVersion.TypedSpec().Value.Version, tt.currentVersion)
			}
		})
	}
}
