// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package tokenfile_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"

	"github.com/siderolabs/omni/internal/pkg/imagefactory/tokenfile"
)

func writeToken(t *testing.T, path, token string) {
	t.Helper()

	// The way a rotation writes it: a new file renamed over the old one.
	tmp := path + ".tmp"

	require.NoError(t, os.WriteFile(tmp, []byte(token+"\n"), 0o600))
	require.NoError(t, os.Rename(tmp, path))
}

func TestLoad(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "token")

	_, err := tokenfile.Load("https://factory.example.org", path, zaptest.NewLogger(t))
	require.Error(t, err, "a missing file fails at startup")

	require.NoError(t, os.WriteFile(path, []byte(" \n"), 0o600))

	_, err = tokenfile.Load("https://factory.example.org", path, zaptest.NewLogger(t))
	require.ErrorContains(t, err, "empty")

	writeToken(t, path, "first")

	token, err := tokenfile.Load("https://factory.example.org", path, zaptest.NewLogger(t))
	require.NoError(t, err)
	require.Equal(t, "first", token.Get(), "surrounding whitespace is trimmed")
}

func TestReloadKeepsTheTokenOnAnEmptyFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "token")
	writeToken(t, path, "first")

	token, err := tokenfile.Load("https://factory.example.org", path, zaptest.NewLogger(t))
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(path, nil, 0o600))

	require.ErrorContains(t, token.Reload(), "empty")
	require.Equal(t, "first", token.Get())

	select {
	case <-token.Changed():
		t.Fatal("an empty file must not count as a change")
	default:
	}

	writeToken(t, path, "second")

	require.NoError(t, token.Reload())
	require.Equal(t, "second", token.Get())

	select {
	case <-token.Changed():
	default:
		t.Fatal("the change was not raised")
	}
}

func TestRunFollowsTheFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "token")
	writeToken(t, path, "first")

	token, err := tokenfile.Load("https://factory.example.org", path, zaptest.NewLogger(t))
	require.NoError(t, err)

	// The write below may land before the watcher is registered, which is exactly what the poll
	// fallback is for.
	token.SetPollInterval(100 * time.Millisecond)

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	var eg errgroup.Group

	eg.Go(func() error { return token.Run(ctx) })

	writeToken(t, path, "second")

	select {
	case <-token.Changed():
	case <-time.After(10 * time.Second):
		t.Fatal("the token change was not noticed")
	}

	require.Equal(t, "second", token.Get())

	// A change that lands while nobody is waiting is not lost: the consumer that reads the token and
	// only then waits still gets it.
	writeToken(t, path, "third")

	require.Eventually(t, func() bool { return token.Get() == "third" }, 10*time.Second, 10*time.Millisecond)

	select {
	case <-token.Changed():
	default:
		t.Fatal("the change raised while nobody was waiting was lost")
	}

	cancel()
	require.NoError(t, eg.Wait())
}

func TestSetFansInChanges(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeToken(t, filepath.Join(dir, "a"), "a1")
	writeToken(t, filepath.Join(dir, "b"), "b1")

	a, err := tokenfile.Load("https://a.example.org", filepath.Join(dir, "a"), zaptest.NewLogger(t))
	require.NoError(t, err)

	b, err := tokenfile.Load("https://b.example.org", filepath.Join(dir, "b"), zaptest.NewLogger(t))
	require.NoError(t, err)

	set := tokenfile.NewSet(a, b)

	a.SetPollInterval(100 * time.Millisecond)
	b.SetPollInterval(100 * time.Millisecond)

	require.Equal(t, "a1", set.Get("https://a.example.org"))
	require.Empty(t, set.Get("https://c.example.org"))

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	var eg errgroup.Group

	eg.Go(func() error { return set.Run(ctx) })

	writeToken(t, filepath.Join(dir, "b"), "b2")

	select {
	case <-set.Changed():
	case <-time.After(10 * time.Second):
		t.Fatal("the set did not report the change of one of its tokens")
	}

	require.Equal(t, "b2", set.Get("https://b.example.org"))

	// Both tokens changing while nobody waits: one signal is pending afterwards, and both values are current.
	writeToken(t, filepath.Join(dir, "a"), "a2")
	writeToken(t, filepath.Join(dir, "b"), "b3")

	require.Eventually(t, func() bool {
		return set.Get("https://a.example.org") == "a2" && set.Get("https://b.example.org") == "b3"
	}, 10*time.Second, 10*time.Millisecond)

	select {
	case <-set.Changed():
	default:
		t.Fatal("the changes raised while nobody was waiting were lost")
	}

	cancel()
	require.NoError(t, eg.Wait())
}

// TestRunFollowsASecretMount covers the way a Kubernetes secret mount rotates: the file is a symlink
// through a "..data" directory symlink, and a rotation writes a new data directory and swaps the
// "..data" symlink, never touching the file's own name.
func TestRunFollowsASecretMount(t *testing.T) {
	t.Parallel()

	mount := t.TempDir()

	writeData := func(version, token string) {
		dir := filepath.Join(mount, "..data_"+version)
		require.NoError(t, os.Mkdir(dir, 0o700))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "token"), []byte(token), 0o600))

		// The swap: a new "..data" symlink renamed over the old one.
		require.NoError(t, os.Symlink("..data_"+version, filepath.Join(mount, "..data.tmp")))
		require.NoError(t, os.Rename(filepath.Join(mount, "..data.tmp"), filepath.Join(mount, "..data")))
	}

	writeData("1", "first")
	require.NoError(t, os.Symlink(filepath.Join("..data", "token"), filepath.Join(mount, "token")))

	token, err := tokenfile.Load("https://factory.example.org", filepath.Join(mount, "token"), zaptest.NewLogger(t))
	require.NoError(t, err)
	require.Equal(t, "first", token.Get())

	token.SetPollInterval(100 * time.Millisecond)

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	var eg errgroup.Group

	eg.Go(func() error { return token.Run(ctx) })

	writeData("2", "second")

	select {
	case <-token.Changed():
	case <-time.After(10 * time.Second):
		t.Fatal("the secret mount rotation was not noticed")
	}

	require.Equal(t, "second", token.Get())

	cancel()
	require.NoError(t, eg.Wait())
}
