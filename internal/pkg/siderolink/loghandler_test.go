// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package siderolink_test

import (
	"context"
	"database/sql"
	"net/netip"
	"path/filepath"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/siderolabs/gen/optional"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/omni/internal/backend/runtime/omni/sqlite"
	"github.com/siderolabs/omni/internal/pkg/config"
	"github.com/siderolabs/omni/internal/pkg/siderolink"
)

func TestLogHandler_HandleMessage(t *testing.T) {
	storageConfig := config.LogsMachine{
		BufferInitialCapacity: 16,
		BufferMaxCapacity:     128,
		BufferSafetyGap:       16,
		Storage: config.LogsMachineStorage{
			NumCompressedChunks: 5,
			FlushPeriod:         time.Second,
			Enabled:             false,
		},
	}

	t.Run("empty log message", func(t *testing.T) {
		machineMap := siderolink.NewMachineMap(&siderolink.MapStorage{})
		st := state.WrapCore(namespaced.NewState(inmem.Build))

		handler, err := siderolink.NewLogHandler(testDB(t), machineMap, st, &storageConfig, zaptest.NewLogger(t))
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		t.Cleanup(cancel)

		handler.HandleMessage(ctx, netip.MustParseAddr("1.2.3.4"), []byte(""))
	})

	t.Run("non-empty log message", func(t *testing.T) {
		cache := siderolink.NewMachineMap(&siderolink.MapStorage{
			IPToMachine: map[string]siderolink.MachineID{
				"1.2.3.4": "machine1",
			},
		})

		st := state.WrapCore(namespaced.NewState(inmem.Build))

		handler, err := siderolink.NewLogHandler(testDB(t), cache, st, &storageConfig, zaptest.NewLogger(t))
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		t.Cleanup(cancel)

		handler.HandleMessage(ctx, netip.MustParseAddr("1.2.3.4"), []byte(`{"hello": "world"}`))
		handler.HandleMessage(ctx, netip.MustParseAddr("1.2.3.4"), []byte(`{"hello": "world2"}`))
		handler.HandleMessage(ctx, netip.MustParseAddr("1.2.3.4"), []byte(`{"hello": "world3"}`))

		reader, err := handler.GetReader(ctx, "machine1", false, optional.None[int32]())
		require.NoError(t, err)

		t.Cleanup(func() {
			require.NoError(t, reader.Close())
		})

		line, err := reader.ReadLine(ctx)
		require.NoError(t, err)
		require.Equal(t, `{"hello": "world"}`, string(line))
		line, err = reader.ReadLine(ctx)
		require.NoError(t, err)
		require.Equal(t, `{"hello": "world2"}`, string(line))
		line, err = reader.ReadLine(ctx)
		require.NoError(t, err)
		require.Equal(t, `{"hello": "world3"}`, string(line))
	})

	t.Run("non-empty log messages with tail 2", func(t *testing.T) {
		cache := siderolink.NewMachineMap(&siderolink.MapStorage{
			IPToMachine: map[string]siderolink.MachineID{
				"1.2.3.4": "machine1",
			},
		})

		st := state.WrapCore(namespaced.NewState(inmem.Build))

		handler, err := siderolink.NewLogHandler(testDB(t), cache, st, &storageConfig, zaptest.NewLogger(t))
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		t.Cleanup(cancel)

		handler.HandleMessage(ctx, netip.MustParseAddr("1.2.3.4"), []byte(`{"hello": "world"}`))
		handler.HandleMessage(ctx, netip.MustParseAddr("1.2.3.4"), []byte(`{"hello": "world2"}`))
		handler.HandleMessage(ctx, netip.MustParseAddr("1.2.3.4"), []byte(`{"hello": "world3"}`))
		handler.HandleMessage(ctx, netip.MustParseAddr("1.2.3.4"), []byte(`{"hello": "world4"}`))
		handler.HandleMessage(ctx, netip.MustParseAddr("1.2.3.4"), []byte(`{"hello": "world5"}`))

		reader, err := handler.GetReader(ctx, "machine1", false, optional.Some[int32](2))
		require.NoError(t, err)

		t.Cleanup(func() {
			require.NoError(t, reader.Close())
		})

		line, err := reader.ReadLine(ctx)
		require.NoError(t, err)
		require.Equal(t, `{"hello": "world4"}`, string(line))
		line, err = reader.ReadLine(ctx)
		require.NoError(t, err)
		require.Equal(t, `{"hello": "world5"}`, string(line))
	})
}

func testDB(t *testing.T) *sql.DB {
	t.Helper()

	conf := config.Default().Storage.SQLite
	conf.Path = filepath.Join(t.TempDir(), "test.db")

	db, err := sqlite.OpenDB(conf)
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, db.Close())
	})

	return db
}
