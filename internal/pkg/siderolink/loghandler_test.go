// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package siderolink_test

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/netip"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/siderolabs/gen/optional"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
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

		handler, err := siderolink.NewLogHandler(machineMap, st, &storageConfig, zaptest.NewLogger(t))
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
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

		handler, err := siderolink.NewLogHandler(cache, st, &storageConfig, zaptest.NewLogger(t))
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
		t.Cleanup(cancel)

		handler.HandleMessage(ctx, netip.MustParseAddr("1.2.3.4"), []byte(`{"hello": "world"}`))
		handler.HandleMessage(ctx, netip.MustParseAddr("1.2.3.4"), []byte(`{"hello": "world2"}`))
		handler.HandleMessage(ctx, netip.MustParseAddr("1.2.3.4"), []byte(`{"hello": "world3"}`))
		reader, err := handler.GetReader(ctx, "machine1", false, optional.None[int32]())
		require.NoError(t, err)
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

		handler, err := siderolink.NewLogHandler(cache, st, &storageConfig, zaptest.NewLogger(t))
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
		t.Cleanup(cancel)

		handler.HandleMessage(ctx, netip.MustParseAddr("1.2.3.4"), []byte(`{"hello": "world"}`))
		handler.HandleMessage(ctx, netip.MustParseAddr("1.2.3.4"), []byte(`{"hello": "world2"}`))
		handler.HandleMessage(ctx, netip.MustParseAddr("1.2.3.4"), []byte(`{"hello": "world3"}`))
		handler.HandleMessage(ctx, netip.MustParseAddr("1.2.3.4"), []byte(`{"hello": "world4"}`))
		handler.HandleMessage(ctx, netip.MustParseAddr("1.2.3.4"), []byte(`{"hello": "world5"}`))

		reader, err := handler.GetReader(ctx, "machine1", false, optional.Some[int32](2))
		require.NoError(t, err)
		line, err := reader.ReadLine(ctx)
		require.NoError(t, err)
		require.Equal(t, `{"hello": "world4"}`, string(line))
		line, err = reader.ReadLine(ctx)
		require.NoError(t, err)
		require.Equal(t, `{"hello": "world5"}`, string(line))
	})
}

// TestLogHandlerStorage tests that log handler can store logs on the filesystem when log storage is enabled.
func TestLogHandlerStorage(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	t.Cleanup(cancel)

	machineMap := siderolink.NewMachineMap(&siderolink.MapStorage{
		IPToMachine: map[string]siderolink.MachineID{
			"1.2.3.4": "machine-1",
			"2.3.4.5": "machine-2",
		},
	})

	st := state.WrapCore(namespaced.NewState(inmem.Build))

	require.NoError(t, st.Create(ctx, omni.NewMachine(resources.DefaultNamespace, "machine-1")))
	require.NoError(t, st.Create(ctx, omni.NewMachine(resources.DefaultNamespace, "machine-2")))

	tempDir := t.TempDir()
	logger := zaptest.NewLogger(t)

	logHandler, err := siderolink.NewLogHandler(machineMap, st, &config.LogsMachine{
		Storage: config.LogsMachineStorage{
			Enabled:             true,
			Path:                tempDir,
			FlushPeriod:         2 * time.Second,
			NumCompressedChunks: 5,
		},
		BufferInitialCapacity: 16,
		BufferMaxCapacity:     128,
		BufferSafetyGap:       16,
	}, logger)
	require.NoError(t, err)

	var eg errgroup.Group

	logger.Info("start log handler", zap.String("tempDir", tempDir))

	eg.Go(func() error { return logHandler.Start(ctx) })

	data, err := io.ReadAll(io.LimitReader(rand.Reader, 120))
	require.NoError(t, err)

	logHandler.HandleMessage(ctx, netip.MustParseAddr("1.2.3.4"), data)
	logHandler.HandleMessage(ctx, netip.MustParseAddr("2.3.4.5"), data)

	// wait for a log flush to happen
	time.Sleep(2300 * time.Millisecond)

	assert.FileExists(t, filepath.Join(tempDir, "machine-1.log.0"))
	assert.FileExists(t, filepath.Join(tempDir, "machine-2.log.0"))
	assert.NoFileExists(t, filepath.Join(tempDir, "machine-1.log.1"))
	assert.NoFileExists(t, filepath.Join(tempDir, "machine-2.log.1"))

	// send more logs
	data = data[:16]

	logHandler.HandleMessage(ctx, netip.MustParseAddr("1.2.3.4"), data)
	logHandler.HandleMessage(ctx, netip.MustParseAddr("2.3.4.5"), data)

	time.Sleep(2300 * time.Millisecond)

	assert.FileExists(t, filepath.Join(tempDir, "machine-1.log.1"))
	assert.FileExists(t, filepath.Join(tempDir, "machine-2.log.1"))

	logHandler.HandleMessage(ctx, netip.MustParseAddr("1.2.3.4"), []byte("bbbb"))
	logHandler.HandleMessage(ctx, netip.MustParseAddr("2.3.4.5"), []byte("BBBB"))

	// destroy machine-2
	require.NoError(t, st.Destroy(ctx, omni.NewMachine(resources.DefaultNamespace, "machine-2").Metadata()))

	// ensure that log and hash files for machine-2 are removed
	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		matches, matchErr := filepath.Glob(filepath.Join(tempDir, "machine-2.log*"))
		require.NoError(t, matchErr)

		assert.Empty(collect, matches)
	}, 10*time.Second, 100*time.Millisecond)

	cancel()

	require.NoError(t, eg.Wait())
}

func TestLogHandlerStorageLegacyMigration(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	t.Cleanup(cancel)

	machineMap := siderolink.NewMachineMap(&siderolink.MapStorage{
		IPToMachine: map[string]siderolink.MachineID{"1.2.3.4": "machine-1"},
	})

	st := state.WrapCore(namespaced.NewState(inmem.Build))

	require.NoError(t, st.Create(ctx, omni.NewMachine(resources.DefaultNamespace, "machine-1")))

	tempDir := t.TempDir()

	// prepare legacy log file
	legacyLogPath := filepath.Join(tempDir, "machine-1.log")
	legacyLogHashPath := legacyLogPath + ".sha256sum"

	legacyLog := []byte("\nlegacy log") // start with an empty line, so it can be parsed as a valid log message
	legacyHash := sha256.Sum256(legacyLog)

	require.NoError(t, os.WriteFile(legacyLogPath, legacyLog, 0o644))
	require.NoError(t, os.WriteFile(legacyLogHashPath, []byte(hex.EncodeToString(legacyHash[:])), 0o644))

	logHandler, err := siderolink.NewLogHandler(machineMap, st, &config.LogsMachine{
		Storage: config.LogsMachineStorage{
			Enabled:             true,
			Path:                tempDir,
			FlushPeriod:         1 * time.Second,
			NumCompressedChunks: 5,
		},
		BufferInitialCapacity: 16,
		BufferMaxCapacity:     128,
		BufferSafetyGap:       16,
	}, zaptest.NewLogger(t))
	require.NoError(t, err)

	var eg errgroup.Group

	eg.Go(func() error { return logHandler.Start(ctx) })

	// write a log to trigger migration
	logHandler.HandleMessage(ctx, netip.MustParseAddr("1.2.3.4"), []byte("foo\n"))

	// assert that the legacy log files are removed
	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		assert.NoFileExists(collect, legacyLogPath)
		assert.NoFileExists(collect, legacyLogHashPath)
	}, 2*time.Second, 50*time.Millisecond)

	reader, err := logHandler.GetReader(ctx, "machine-1", false, optional.None[int32]())
	require.NoError(t, err)

	t.Cleanup(func() { require.NoError(t, reader.Close()) })

	line, err := reader.ReadLine(ctx)
	require.NoError(t, err)

	assert.Equal(t, "legacy log", string(line))
}

// TestLogHandlerStorageDisabled ensures that log handler works as expected without log storage.
func TestLogHandlerStorageDisabled(t *testing.T) {
	ctx, ctxCancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer ctxCancel()

	machineMap := siderolink.NewMachineMap(&siderolink.MapStorage{
		IPToMachine: map[string]siderolink.MachineID{
			"1.2.3.4": "machine-1",
			"2.3.4.5": "machine-2",
		},
	})

	st := state.WrapCore(namespaced.NewState(inmem.Build))

	require.NoError(t, st.Create(ctx, omni.NewMachine(resources.DefaultNamespace, "machine-1")))
	require.NoError(t, st.Create(ctx, omni.NewMachine(resources.DefaultNamespace, "machine-2")))

	tempDir := t.TempDir()
	storageConfig := config.LogsMachine{
		Storage: config.LogsMachineStorage{
			Enabled:     false,
			Path:        tempDir,
			FlushPeriod: 100 * time.Millisecond,
		},
	}

	handler, err := siderolink.NewLogHandler(machineMap, st, &storageConfig, zaptest.NewLogger(t))
	require.NoError(t, err)

	var eg errgroup.Group

	eg.Go(func() error { return handler.Start(ctx) })

	time.Sleep(300 * time.Millisecond)

	handler.HandleMessage(ctx, netip.MustParseAddr("1.2.3.4"), []byte("aaaa"))
	handler.HandleMessage(ctx, netip.MustParseAddr("2.3.4.5"), []byte("AAAA"))

	ctxCancel()

	require.NoError(t, eg.Wait())

	assert.NoFileExists(t, filepath.Join(tempDir, "machine-1.log"))
	assert.NoFileExists(t, filepath.Join(tempDir, "machine-1.log.sha256sum"))

	assert.NoFileExists(t, filepath.Join(tempDir, "machine-2.log"))
	assert.NoFileExists(t, filepath.Join(tempDir, "machine-2.log.sha256sum"))
}
