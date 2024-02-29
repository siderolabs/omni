// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package siderolink

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// LogStorage stores logs for machines on file system.
type LogStorage struct {
	Path string
}

// NewLogStorage creates a new LogStorage.
func NewLogStorage(path string) *LogStorage {
	return &LogStorage{
		Path: path,
	}
}

// Exists returns true if the log file exists for the given machine ID.
func (l *LogStorage) Exists(id MachineID) (bool, error) {
	_, err := os.Stat(l.logPath(id))
	if err == nil {
		return true, nil
	}

	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}

	return false, err
}

// Load loads logs of the machine with the given id, if exists, into the given writer.
func (l *LogStorage) Load(id MachineID, writer io.Writer) error {
	filePath := l.logPath(id)

	bufferData, err := os.ReadFile(filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}

		return err
	}

	hashHexExpectedBytes, err := os.ReadFile(l.logSHA256Path(id))
	if err != nil {
		removeErr := l.Remove(id)
		if removeErr != nil {
			return fmt.Errorf("failed to delete invalid log buffer file: %w", removeErr)
		}

		return fmt.Errorf("failed to read log buffer hash file: %w", err)
	}

	hashHexExpected := string(hashHexExpectedBytes)

	// verify the hash
	hashActual := sha256.Sum256(bufferData)
	hashHexActual := hex.EncodeToString(hashActual[:])

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

// Remove removes the log file, if exists, for the given machine ID.
func (l *LogStorage) Remove(id MachineID) error {
	if err := l.removeFileIfExists(l.logPath(id)); err != nil {
		return err
	}

	return l.removeFileIfExists(l.logSHA256Path(id))
}

// Save saves the given reader into the log file for the given machine ID.
func (l *LogStorage) Save(machineID MachineID, reader io.Reader) error {
	err := os.MkdirAll(l.Path, 0o755)
	if err != nil {
		return fmt.Errorf("failed to create log storage directory: %w", err)
	}

	logFilePath := l.logPath(machineID)

	file, err := os.OpenFile(logFilePath, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("failed to open log file for machine '%s': %w", machineID, err)
	}

	defer file.Close() //nolint:errcheck

	hash := sha256.New()

	teeReader := io.TeeReader(reader, hash)

	_, err = io.Copy(file, teeReader)
	if err != nil {
		return fmt.Errorf("failed to copy buffer to file for machine '%s': %w", machineID, err)
	}

	hashHex := hex.EncodeToString(hash.Sum(nil))

	return os.WriteFile(l.logSHA256Path(machineID), []byte(hashHex), 0o644)
}

func (l *LogStorage) logPath(machineID MachineID) string {
	return filepath.Join(l.Path, fmt.Sprintf("%s.log", machineID))
}

func (l *LogStorage) logSHA256Path(machineID MachineID) string {
	return l.logPath(machineID) + ".sha256sum"
}

func (l *LogStorage) removeFileIfExists(path string) error {
	err := os.Remove(path)
	if err == nil || errors.Is(err, os.ErrNotExist) {
		return nil
	}

	return err
}
