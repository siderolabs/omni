// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
package discovery_test

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/internal/backend/discovery"
	"github.com/siderolabs/omni/internal/pkg/config"
)

func TestInitSQLiteSnapshotStore(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	t.Cleanup(cancel)

	_, db := setupStore(ctx, t)

	var conf config.EmbeddedDiscoveryService
	conf.SetSnapshotsEnabled(true)

	store, err := discovery.InitSQLiteSnapshotStore(ctx, conf, db)
	require.NoError(t, err)

	// Verify SQLite store is empty initially
	rdr, err := store.Reader(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, rdr.Close()) })

	data, err := io.ReadAll(rdr)
	require.NoError(t, err)
	assert.Empty(t, data)
}
