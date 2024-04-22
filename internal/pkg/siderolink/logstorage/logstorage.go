// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package logstorage implements storage for storing logs of machines on the filesystem.
package logstorage

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/hashicorp/go-multierror"
	"github.com/klauspost/compress/zstd"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"
)

// CompressionExtension is the extension used for compressed log files.
const CompressionExtension = "zst"

// Storage stores logs for machines on file system.
type Storage struct {
	dirtyMachineIDs map[string]struct{}
	logger          *zap.Logger
	Path            string
	Compress        bool
	lock            sync.Mutex
}

// New creates a new Storage.
func New(path string, compress bool, logger *zap.Logger) *Storage {
	return &Storage{
		Path:            path,
		Compress:        compress,
		dirtyMachineIDs: map[string]struct{}{},
		logger:          logger,
	}
}

// MarkDirty marks the machine with the given ID as dirty.
func (l *Storage) MarkDirty(id string) {
	l.lock.Lock()
	defer l.lock.Unlock()
	l.dirtyMachineIDs[id] = struct{}{}
}

// Exists returns true if the log file exists for the given machine ID.
func (l *Storage) Exists(id string) (bool, error) {
	_, err := os.Stat(l.logPath(id, l.Compress))
	if err == nil {
		return true, nil
	}

	if !errors.Is(err, os.ErrNotExist) {
		return false, err
	}

	_, err = os.Stat(l.logPath(id, !l.Compress))
	if err == nil {
		return true, nil
	}

	if !errors.Is(err, os.ErrNotExist) {
		return false, err
	}

	return false, nil
}

// Load loads logs of the machine with the given id, if exists, into the given writer.
func (l *Storage) Load(id string, writer io.Writer) error {
	compressed := l.Compress

	bufferDataOptional, hashHexActual, err := l.readLogFile(id, l.Compress)
	if err != nil {
		return err
	}

	if !bufferDataOptional.IsPresent() {
		if bufferDataOptional, hashHexActual, err = l.readLogFile(id, !l.Compress); err != nil {
			return err
		}

		compressed = !compressed
	}

	bufferData, ok := bufferDataOptional.Get()
	if !ok {
		return nil
	}

	hashHexExpectedBytes, err := os.ReadFile(l.logSHA256Path(id, compressed))
	if err != nil {
		removeErr := l.Remove(id)
		if removeErr != nil {
			return fmt.Errorf("failed to delete invalid log buffer file: %w", removeErr)
		}

		return fmt.Errorf("failed to read log buffer hash file: %w", err)
	}

	hashHexExpected := string(hashHexExpectedBytes)

	// verify the hash
	if hashHexExpected != hashHexActual {
		removeErr := l.Remove(id)
		if removeErr != nil {
			return fmt.Errorf("failed to delete invalid log buffer file: %w", removeErr)
		}

		return fmt.Errorf("invalid log buffer hash in file: expected %s, got %s", hashHexExpected, hashHexActual)
	}

	_, err = io.Copy(writer, bytes.NewReader(bufferData))

	return err
}

func (l *Storage) readLogFile(id string, compressed bool) (optional.Optional[[]byte], string, error) {
	path := l.logPath(id, compressed)

	bufferData, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return optional.None[[]byte](), "", nil
		}

		return optional.None[[]byte](), "", err
	}

	hash := sha256.New()
	hashingReader := io.TeeReader(bytes.NewReader(bufferData), hash)

	reader := hashingReader

	if compressed {
		reader, err = zstd.NewReader(hashingReader)
		if err != nil {
			return optional.None[[]byte](), "", err
		}
	}

	bufferData, err = io.ReadAll(reader)
	if err != nil {
		return optional.None[[]byte](), "", err
	}

	hashHex := hex.EncodeToString(hash.Sum(nil))

	return optional.Some(bufferData), hashHex, nil
}

// Remove removes the log file, if exists, for the given machine ID.
func (l *Storage) Remove(id string) error {
	var errs error

	if err := l.remove(id, true); err != nil {
		errs = multierror.Append(errs, err)
	}

	if err := l.remove(id, false); err != nil {
		errs = multierror.Append(errs, err)
	}

	return errs
}

func (l *Storage) remove(id string, compressed bool) error {
	var errs error

	if err := l.removeFileIfExists(l.logPath(id, compressed)); err != nil {
		errs = multierror.Append(errs, err)
	}

	if err := l.removeFileIfExists(l.logSHA256Path(id, compressed)); err != nil {
		errs = multierror.Append(errs, err)
	}

	return errs
}

// Save saves the given reader into the log file for the given machine ID.
func (l *Storage) Save(machineID string, reader io.Reader) error {
	removeObsoleteLogFiles := func() error {
		// if the log is not dirty, we don't need to save it, only ensure that the !l.Compress log
		// (e.g., if Omni was run with disabled compression on the previous run, but now it is enabled, or vice versa) is removed if exists and exit
		return l.remove(machineID, !l.Compress)
	}

	l.lock.Lock()
	_, dirty := l.dirtyMachineIDs[machineID]
	l.lock.Unlock()

	if !dirty {
		l.logger.Info("skip saving logs for machine as it is not dirty", zap.String("machine_id", machineID))

		return removeObsoleteLogFiles()
	}

	l.logger.Info("save logs for machine", zap.String("machine_id", machineID))

	err := os.MkdirAll(l.Path, 0o755)
	if err != nil {
		return fmt.Errorf("failed to create log storage directory: %w", err)
	}

	logFilePath := l.logPath(machineID, l.Compress)

	var (
		file              io.WriteCloser
		compressingWriter io.WriteCloser
	)

	closeAll := func() error {
		var errs error

		if compressingWriter != nil {
			if closeErr := compressingWriter.Close(); closeErr != nil {
				errs = multierror.Append(errs, fmt.Errorf("failed to close compressing writer: %w", closeErr))
			}
		}

		if file != nil {
			if closeErr := file.Close(); closeErr != nil {
				errs = multierror.Append(errs, fmt.Errorf("failed to close log file for machine '%s': %w", machineID, closeErr))
			}
		}

		return errs
	}

	defer closeAll() //nolint:errcheck

	file, err = os.OpenFile(logFilePath, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("failed to open log file for machine '%s': %w", machineID, err)
	}

	hash := sha256.New()
	hashingWriter := io.MultiWriter(file, hash)

	if l.Compress {
		compressingWriter, err = zstd.NewWriter(hashingWriter)
		if err != nil {
			return fmt.Errorf("failed to create compressing writer: %w", err)
		}

		hashingWriter = compressingWriter
	}

	if _, err = io.Copy(hashingWriter, reader); err != nil {
		return fmt.Errorf("failed to write log file for machine '%s': %w", machineID, err)
	}

	if err = closeAll(); err != nil { // close the writers to flush any remaining data before computing the checksum
		return err
	}

	hashHex := hex.EncodeToString(hash.Sum(nil))

	if err = os.WriteFile(l.logSHA256Path(machineID, l.Compress), []byte(hashHex), 0o644); err != nil {
		return err
	}

	// clear the dirty status after successful save
	l.lock.Lock()
	delete(l.dirtyMachineIDs, machineID)
	l.lock.Unlock()

	return removeObsoleteLogFiles()
}

func (l *Storage) logPath(machineID string, compressed bool) string {
	path := filepath.Join(l.Path, fmt.Sprintf("%s.log", machineID))

	if compressed {
		path += "." + CompressionExtension
	}

	return path
}

func (l *Storage) logSHA256Path(machineID string, compressed bool) string {
	return l.logPath(machineID, compressed) + ".sha256sum"
}

func (l *Storage) removeFileIfExists(path string) error {
	err := os.Remove(path)
	if err == nil || errors.Is(err, os.ErrNotExist) {
		return nil
	}

	return err
}
