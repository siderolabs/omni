// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package siderolink_test

import (
	"context"
	"errors"
	"net/netip"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/xtesting/must"
	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/pkg/config"
	"github.com/siderolabs/omni/internal/pkg/siderolink"
)

func TestLogHandler_HandleMessage(t *testing.T) {
	t.Run("empty log message", func(t *testing.T) {
		machineMap := siderolink.NewMachineMap(&siderolink.MapStorage{})

		st := state.WrapCore(namespaced.NewState(inmem.Build))
		storageConfig := config.LogStorageParams{
			Enabled: false,
		}

		handler := siderolink.NewLogHandler(machineMap, st, &storageConfig, zaptest.NewLogger(t))
		handler.HandleMessage(netip.MustParseAddr("1.2.3.4"), []byte(""))
	})

	t.Run("non-empty log message", func(t *testing.T) {
		cache := siderolink.NewMachineMap(&siderolink.MapStorage{
			IPToMachine: map[string]siderolink.MachineID{
				"1.2.3.4": "machine1",
			},
		})

		st := state.WrapCore(namespaced.NewState(inmem.Build))
		storageConfig := config.LogStorageParams{
			Enabled: false,
		}

		handler := siderolink.NewLogHandler(cache, st, &storageConfig, zaptest.NewLogger(t))
		handler.HandleMessage(netip.MustParseAddr("1.2.3.4"), []byte(`{"hello": "world"}`))
		handler.HandleMessage(netip.MustParseAddr("1.2.3.4"), []byte(`{"hello": "world2"}`))
		handler.HandleMessage(netip.MustParseAddr("1.2.3.4"), []byte(`{"hello": "world3"}`))
		reader, err := handler.GetReader("machine1", false, optional.None[int32]())
		require.NoError(t, err)
		line, err := reader.ReadLine()
		require.NoError(t, err)
		require.Equal(t, `{"hello": "world"}`, string(line))
		line, err = reader.ReadLine()
		require.NoError(t, err)
		require.Equal(t, `{"hello": "world2"}`, string(line))
		line, err = reader.ReadLine()
		require.NoError(t, err)
		require.Equal(t, `{"hello": "world3"}`, string(line))
	})

	t.Run("non-empty log message with invalid data in the beginning", func(t *testing.T) {
		cache := siderolink.NewMachineMap(&siderolink.MapStorage{
			IPToMachine: map[string]siderolink.MachineID{
				"1.2.3.4": "machine1",
			},
		})

		st := state.WrapCore(namespaced.NewState(inmem.Build))
		storageConfig := config.LogStorageParams{
			Enabled: false,
		}

		handler := siderolink.NewLogHandler(cache, st, &storageConfig, zaptest.NewLogger(t))
		writeBytes(t, handler, "1.2.3.4", []byte(`{"hello": "first"}`))

		handler.HandleMessage(netip.MustParseAddr("1.2.3.4"), []byte(`{"hello": "world"}`))
		handler.HandleMessage(netip.MustParseAddr("1.2.3.4"), []byte(`{"hello": "world2"}`))
		handler.HandleMessage(netip.MustParseAddr("1.2.3.4"), []byte(`{"hello": "world3"}`))
		reader, err := handler.GetReader("machine1", false, optional.None[int32]())
		require.NoError(t, err)
		line, err := reader.ReadLine()
		require.NoError(t, err)
		require.Equal(t, `{"hello": "world2"}`, string(line))
		line, err = reader.ReadLine()
		require.NoError(t, err)
		require.Equal(t, `{"hello": "world3"}`, string(line))
	})

	t.Run("non-empty log message with invalid message in the beginning", func(t *testing.T) {
		cache := siderolink.NewMachineMap(&siderolink.MapStorage{
			IPToMachine: map[string]siderolink.MachineID{
				"1.2.3.4": "machine1",
			},
		})

		st := state.WrapCore(namespaced.NewState(inmem.Build))
		storageConfig := config.LogStorageParams{
			Enabled: false,
		}

		handler := siderolink.NewLogHandler(cache, st, &storageConfig, zaptest.NewLogger(t))
		writeBytes(t, handler, "1.2.3.4", []byte(`{"hello": "first"}`+"\n"))

		handler.HandleMessage(netip.MustParseAddr("1.2.3.4"), []byte(`{"hello": "world"}`))
		handler.HandleMessage(netip.MustParseAddr("1.2.3.4"), []byte(`{"hello": "world2"}`))
		handler.HandleMessage(netip.MustParseAddr("1.2.3.4"), []byte(`{"hello": "world3"}`))
		reader, err := handler.GetReader("machine1", false, optional.None[int32]())
		require.NoError(t, err)
		line, err := reader.ReadLine()
		require.NoError(t, err)
		require.Equal(t, `{"hello": "world"}`, string(line))
		line, err = reader.ReadLine()
		require.NoError(t, err)
		require.Equal(t, `{"hello": "world2"}`, string(line))
		line, err = reader.ReadLine()
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
		storageConfig := config.LogStorageParams{
			Enabled: false,
		}

		handler := siderolink.NewLogHandler(cache, st, &storageConfig, zaptest.NewLogger(t))
		handler.HandleMessage(netip.MustParseAddr("1.2.3.4"), []byte(`{"hello": "world"}`))
		handler.HandleMessage(netip.MustParseAddr("1.2.3.4"), []byte(`{"hello": "world2"}`))
		handler.HandleMessage(netip.MustParseAddr("1.2.3.4"), []byte(`{"hello": "world3"}`))
		handler.HandleMessage(netip.MustParseAddr("1.2.3.4"), []byte(`{"hello": "world4"}`))
		handler.HandleMessage(netip.MustParseAddr("1.2.3.4"), []byte(`{"hello": "world5"}`))

		reader, err := handler.GetReader("machine1", false, optional.Some[int32](2))
		require.NoError(t, err)
		line, err := reader.ReadLine()
		require.NoError(t, err)
		require.Equal(t, `{"hello": "world4"}`, string(line))
		line, err = reader.ReadLine()
		require.NoError(t, err)
		require.Equal(t, `{"hello": "world5"}`, string(line))
	})
}

// TestLogHandlerStorage tests that log handler can store logs on the filesystem when log storage is enabled.
func TestLogHandlerStorage(t *testing.T) {
	ctx, ctxCancel := context.WithTimeout(context.Background(), 10*time.Second)
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

	logHandler := siderolink.NewLogHandler(machineMap, st, &config.LogStorageParams{
		Enabled:     true,
		Path:        tempDir,
		FlushPeriod: 2 * time.Second,
	}, zaptest.NewLogger(t))

	var eg errgroup.Group

	eg.Go(func() error { return logHandler.Start(ctx) })

	logHandler.HandleMessage(netip.MustParseAddr("1.2.3.4"), []byte("aaaa"))
	logHandler.HandleMessage(netip.MustParseAddr("2.3.4.5"), []byte("AAAA"))

	// wait for a log flush to happen
	time.Sleep(2100 * time.Millisecond)

	assertLogExists(t, tempDir, "machine-1", "aaaa")
	assertLogExists(t, tempDir, "machine-2", "AAAA")

	// destroy machine-2
	require.NoError(t, st.Destroy(ctx, omni.NewMachine(resources.DefaultNamespace, "machine-2").Metadata()))

	// ensure that log and hash files for machine-2 are removed
	require.NoError(t, retry.Constant(10*time.Second).Retry(func() error {
		logFileExists, err := fileExists(filepath.Join(tempDir, "machine-2.log"))
		require.NoError(t, err)

		if logFileExists {
			return retry.ExpectedError(errors.New("log file for machine-2 is still present"))
		}

		logHashExists, err := fileExists(filepath.Join(tempDir, "machine-2.log.sha256sum"))
		require.NoError(t, err)

		if logHashExists {
			return retry.ExpectedError(errors.New("log hash for machine-2 is still present"))
		}

		return nil
	}), "log and hash files for machine-2 should be removed")

	// send another log and immediately cancel the context
	logHandler.HandleMessage(netip.MustParseAddr("1.2.3.4"), []byte("bbbb"))

	ctxCancel()

	require.NoError(t, eg.Wait())

	// ensure that the last log entry is flushed to disk on shutdown
	assertLogExists(t, tempDir, "machine-1", "bbbb")
}

// TestLogHandlerStorageDisabled ensures that log handler works as expected without log storage.
func TestLogHandlerStorageDisabled(t *testing.T) {
	ctx, ctxCancel := context.WithTimeout(context.Background(), 5*time.Second)
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
	storageConfig := config.LogStorageParams{
		Enabled:     false,
		Path:        tempDir,
		FlushPeriod: 100 * time.Millisecond,
	}

	handler := siderolink.NewLogHandler(machineMap, st, &storageConfig, zaptest.NewLogger(t))

	var eg errgroup.Group

	eg.Go(func() error { return handler.Start(ctx) })

	time.Sleep(300 * time.Millisecond)

	handler.HandleMessage(netip.MustParseAddr("1.2.3.4"), []byte("aaaa"))
	handler.HandleMessage(netip.MustParseAddr("2.3.4.5"), []byte("AAAA"))

	ctxCancel()

	require.NoError(t, eg.Wait())

	assert.NoFileExists(t, filepath.Join(tempDir, "machine-1.log"))
	assert.NoFileExists(t, filepath.Join(tempDir, "machine-1.log.sha256sum"))

	assert.NoFileExists(t, filepath.Join(tempDir, "machine-2.log"))
	assert.NoFileExists(t, filepath.Join(tempDir, "machine-2.log.sha256sum"))
}

func assertLogExists(t *testing.T, dir, machineID, log string) {
	fileBytes, err := os.ReadFile(filepath.Join(dir, machineID+".log"))
	if err != nil {
		// Print additional directory contents to help with debugging.
		stat := must.Value(os.Stat(dir))(t)
		require.True(t, stat.IsDir(), "expected %q to be a directory", dir)

		readDir := must.Value(os.ReadDir(dir))(t)
		t.Log("directory contents:")

		for _, entry := range readDir {
			t.Logf("%s", entry.Name())
		}

		t.Fatal(err)
	}

	assert.Contains(t, string(fileBytes), log)
}

func writeBytes(t *testing.T, handler *siderolink.LogHandler, ip string, bytes []byte) {
	id := must.Value(handler.Map.GetMachineID(ip))(t)
	writeTo := must.Value(handler.Cache.GetWriter(id))(t)
	must.Value(writeTo.Write(bytes))(t)
}

func fileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}

	return err == nil, err
}
