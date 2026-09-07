// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package tokenfile reads an image factory API token from a file and follows the file as it changes.
//
// The token is passed to Omni as a file rather than a value, so that whoever rotates it (a Kubernetes
// secret, an operator by hand) does not have to restart Omni: the file is re-read when it changes, and
// every reader of the token sees the new one.
package tokenfile

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/panichandler"
)

// DefaultPollInterval is how often the file is re-read regardless of notifications, so that a change
// still gets picked up when the notifications do not fire (some filesystems, some mount types).
const DefaultPollInterval = time.Minute

// Token is a token read from a file, kept up to date by [Token.Run].
type Token struct {
	logger *zap.Logger

	// changed carries one pending signal: a change raised while nobody is waiting stays buffered, so
	// a consumer that looks at the token and then waits never misses the change in between.
	changed chan struct{}

	// notify is the signal of the Set holding the token, if any, raised along with changed.
	notify chan struct{}

	factoryURL string
	path       string
	token      string

	pollInterval time.Duration

	mu sync.RWMutex
}

// Load reads the token of the factory at factoryURL from path and returns a Token following the file.
//
// The initial read must succeed: an Omni configured with a token file that cannot be read has no way
// to reach its factory, which is worth failing at startup rather than on the first request.
func Load(factoryURL, path string, logger *zap.Logger) (*Token, error) {
	t := &Token{
		logger:       logger.With(zap.String("factory", factoryURL), zap.String("path", path)),
		changed:      make(chan struct{}, 1),
		factoryURL:   factoryURL,
		path:         path,
		pollInterval: DefaultPollInterval,
	}

	if err := t.reload(); err != nil {
		return nil, err
	}

	// The initial read is not a change.
	select {
	case <-t.changed:
	default:
	}

	return t, nil
}

// Get returns the current token.
func (t *Token) Get() string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.token
}

// Changed returns a channel that receives when the token changed since the last receive. It is
// meant for a single consumer.
func (t *Token) Changed() <-chan struct{} {
	return t.changed
}

// Run follows the file until ctx is done. The directory is watched rather than the file, since a
// Kubernetes secret mount updates the file by swapping a symlink, which a watch on the file itself
// misses. The file is also re-read on a timer, in case the notifications do not fire at all.
func (t *Token) Run(ctx context.Context) error {
	var (
		events <-chan fsnotify.Event
		errs   <-chan error
	)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.logger.Warn("failed to create a file watcher for the image factory token, polling only", zap.Error(err))
	} else {
		defer watcher.Close() //nolint:errcheck

		if err = watcher.Add(filepath.Dir(t.path)); err != nil {
			t.logger.Warn("failed to watch the directory of the image factory token, polling only", zap.Error(err))
		} else {
			events, errs = watcher.Events, watcher.Errors
		}
	}

	ticker := time.NewTicker(t.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		case event, ok := <-events:
			if !ok {
				// The watcher is gone, the poll carries on alone.
				events, errs = nil, nil

				continue
			}

			// A secret mount rotates by creating the new data dir and swapping the "..data" symlink, so
			// the event may name the symlink rather than the token file. Any event in the directory is
			// reason enough to re-read: the read is cheap and a no-op when nothing changed.
			t.logger.Debug("image factory token directory changed", zap.String("event", event.String()))
		case err, ok := <-errs:
			if !ok {
				events, errs = nil, nil

				continue
			}

			t.logger.Warn("image factory token watcher error", zap.Error(err))

			continue
		}

		if err := t.reload(); err != nil {
			// The file being briefly absent while it is replaced is expected, the previous token stays
			// in use and the next event or tick reads the new one.
			t.logger.Warn("failed to re-read the image factory token, keeping the current one", zap.Error(err))
		}
	}
}

// reload reads the file and raises the change signal when the token changed.
func (t *Token) reload() error {
	data, err := os.ReadFile(t.path)
	if err != nil {
		return fmt.Errorf("failed to read the image factory token file: %w", err)
	}

	token := string(bytes.TrimSpace(data))
	if token == "" {
		// An emptied file is not a rotation, it is a broken one: keep the token that works.
		return fmt.Errorf("the image factory token file is empty")
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if token == t.token {
		return nil
	}

	if t.token != "" {
		t.logger.Info("the image factory token changed")
	}

	t.token = token

	// Raised under the lock, so a consumer that read the new value finds the signal pending too.
	raise(t.changed)
	raise(t.notify)

	return nil
}

// raise leaves a signal pending on ch, unless one already is. A nil ch is no channel to signal.
func raise(ch chan struct{}) {
	select {
	case ch <- struct{}{}:
	default:
	}
}

// Set holds the token files of the configured factories, by the normalized factory URL. It is built
// before Run and before any Get, so the map itself is not guarded.
type Set struct {
	tokens map[string]*Token

	// changed carries one pending signal for any token changing, see Token.changed.
	changed chan struct{}
}

// NewSet builds a Set of the given tokens, nil ones are skipped.
func NewSet(tokens ...*Token) *Set {
	s := &Set{
		tokens:  map[string]*Token{},
		changed: make(chan struct{}, 1),
	}

	for _, token := range tokens {
		if token == nil {
			continue
		}

		token.notify = s.changed
		s.tokens[token.factoryURL] = token
	}

	return s
}

// Get returns the current token of the factory at factoryURL, empty when the factory has none.
func (s *Set) Get(factoryURL string) string {
	if s == nil || s.tokens[factoryURL] == nil {
		return ""
	}

	return s.tokens[factoryURL].Get()
}

// Changed returns a channel that receives when any of the tokens changed since the last receive. A
// nil channel, which never fires, for an empty set. It is meant for a single consumer.
func (s *Set) Changed() <-chan struct{} {
	if s == nil {
		return nil
	}

	return s.changed
}

// Run follows every token file until ctx is done.
func (s *Set) Run(ctx context.Context) error {
	if s == nil || len(s.tokens) == 0 {
		<-ctx.Done()

		return nil
	}

	eg, ctx := panichandler.ErrGroupWithContext(ctx)

	for _, token := range s.tokens {
		eg.Go(func() error { return token.Run(ctx) })
	}

	return eg.Wait()
}
