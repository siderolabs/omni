// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//nolint:staticcheck // circularlog is deprecated, but it is fine here
package circularlog

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/siderolabs/go-circular"
	"github.com/siderolabs/go-tail"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/panichandler"
	"github.com/siderolabs/omni/internal/pkg/config"
	"github.com/siderolabs/omni/internal/pkg/siderolink/logstore"
)

// NewStore creates a new Store.
func NewStore(config *config.LogsMachine, id string, compressor circular.Compressor, logger *zap.Logger) (*Store, error) {
	bufferOpts := []circular.OptionFunc{
		circular.WithInitialCapacity(config.BufferInitialCapacity),
		circular.WithMaxCapacity(config.BufferMaxCapacity),
		circular.WithSafetyGap(config.BufferSafetyGap),
		circular.WithNumCompressedChunks(config.Storage.NumCompressedChunks, compressor),
		circular.WithLogger(logger),
	}

	if config.Storage.Enabled {
		bufferOpts = append(bufferOpts, circular.WithPersistence(circular.PersistenceOptions{
			ChunkPath:     filepath.Join(config.Storage.Path, id+".log"),
			FlushInterval: config.Storage.FlushPeriod,
			FlushJitter:   config.Storage.FlushJitter,
		}))
	}

	buffer, err := circular.NewBuffer(bufferOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create circular buffer for machine %q: %w", id, err)
	}

	if config.Storage.Enabled {
		loadLegacyLogs(config, id, buffer, logger)
	}

	return &Store{buf: buffer, logger: logger}, nil
}

// Store implements the logstore.LogStore interface using a circular buffer as the backend.
type Store struct {
	buf    *circular.Buffer
	logger *zap.Logger
}

// WriteLine implements the logstore.LogStore interface.
func (s *Store) WriteLine(_ context.Context, message []byte) error {
	if _, err := s.buf.Write([]byte("\n")); err != nil {
		return err
	}

	if _, err := s.buf.Write(message); err != nil {
		return err
	}

	if _, err := s.buf.Write([]byte("\n")); err != nil {
		return err
	}

	return nil
}

// Close closes the Store.
func (s *Store) Close() error {
	return s.buf.Close()
}

// Reader implements the logstore.LogStore interface.
func (s *Store) Reader(ctx context.Context, nLines int, follow bool) (logstore.LineReader, error) {
	var rdr io.ReadSeekCloser

	closeCh := make(chan struct{})

	if follow {
		rdr = s.buf.GetStreamingReader()

		// Make sure that we close the reader when the context is done
		panichandler.Go(func() {
			select {
			case <-closeCh: // normal close, reader is already closed
				return
			case <-ctx.Done(): // context is done, close the reader
			}

			if err := rdr.Close(); err != nil {
				s.logger.Error("failed to close circular buffer on context done", zap.Error(err))
			}
		}, s.logger)
	} else {
		rdr = s.buf.GetReader()
	}

	if rdr == nil {
		return nil, fmt.Errorf("buffer reader is not available")
	}

	if nLines > 0 {
		// since we are surrounding each message with \n we should increase lines by two times.
		seekLines := nLines * 2

		if err := tail.SeekLines(rdr, seekLines); err != nil {
			return nil, fmt.Errorf("failed to seek %d lines: %w", seekLines, err)
		}
	}

	return &LineReader{reader: rdr, closeCh: closeCh}, nil
}

// LineReader is a reader which reads lines surrounded by \n from the underlying reader.
type LineReader struct {
	reader    io.ReadCloser
	buf       *bufio.Reader
	closeCh   chan struct{}
	closeOnce sync.Once
}

// Close closes the LineReader underlying reader.
func (r *LineReader) Close() error {
	closeErr := r.reader.Close()

	r.closeOnce.Do(func() {
		close(r.closeCh)
	})

	return closeErr
}

// ReadLine reads a line from the underlying reader.
func (r *LineReader) ReadLine(context.Context) ([]byte, error) {
	if r.buf == nil {
		r.buf = bufio.NewReader(r.reader)
	}

	for {
		emptyLine, err := r.buf.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				return nil, io.EOF
			}

			return nil, fmt.Errorf("failed to read line: %w", err)
		}

		if len(emptyLine) != 1 {
			// missed the start of the line, skipping to the next entry
			continue
		}

		logLine, err := r.buf.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				return nil, io.EOF
			}

			return nil, fmt.Errorf("failed to read line: %w", err)
		}

		return trimNewlines(logLine), nil
	}
}

// trimNewlines trims a newline from the start and from end of a byte slice.
func trimNewlines(data []byte) []byte {
	if len(data) == 0 {
		return data
	}

	if data[0] == '\n' {
		data = data[1:]
	}

	if len(data) > 0 && data[len(data)-1] == '\n' {
		data = data[:len(data)-1]
	}

	return data
}

// loadLegacyLogs loads logs stored of the machine with the given id in the old format, if exists, into the given writer.
// It is used to migrate logs from the old format to the new format.
// It removes the old log file and its hash file regardless of the result.
//
// It is a best-effort function and does not return any error.
func loadLegacyLogs(config *config.LogsMachine, id string, writer io.Writer, logger *zap.Logger) {
	filePath := filepath.Join(config.Storage.Path, fmt.Sprintf("%s.log", id))
	shaSumPath := filePath + ".sha256sum"

	defer func() {
		if err := os.Remove(filePath); err != nil && !errors.Is(err, os.ErrNotExist) {
			logger.Error("failed to remove legacy log file", zap.String("path", filePath), zap.Error(err))
		}

		if err := os.Remove(shaSumPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			logger.Error("failed to remove legacy log hash file", zap.String("path", shaSumPath), zap.Error(err))
		}
	}()

	bufferData, err := os.ReadFile(filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return
		}

		logger.Error("failed to read legacy log buffer file", zap.String("path", filePath), zap.Error(err))

		return
	}

	hashHexExpectedBytes, err := os.ReadFile(shaSumPath)
	if err != nil {
		logger.Error("failed to read legacy log buffer hash file", zap.String("path", shaSumPath), zap.Error(err))

		return
	}

	hashHexExpected := string(hashHexExpectedBytes)

	// verify the hash
	hashActual := sha256.Sum256(bufferData)
	hashHexActual := hex.EncodeToString(hashActual[:])

	if hashHexExpected != hashHexActual {
		logger.Error("invalid legacy log buffer hash in file", zap.String("expected", hashHexExpected), zap.String("actual", hashHexActual))

		return
	}

	if _, err = io.Copy(writer, bytes.NewReader(bufferData)); err != nil {
		logger.Error("failed to write legacy log buffer to writer", zap.Error(err))
	}

	logger.Info("loaded legacy log buffer", zap.String("path", filePath))
}
