// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package logstorage_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/omni/internal/pkg/siderolink/logstorage"
)

type LogStorageSuite struct {
	suite.Suite
}

// TestSkipSaveWhenNotDirty tests that log storage skips saving logs when they are not dirty.
func (l *LogStorageSuite) TestSkipSaveWhenNotDirty() {
	logger := zaptest.NewLogger(l.T())

	tempDir := l.T().TempDir()
	logStorage := logstorage.New(tempDir, false, logger)

	// save log without marking it as dirty - it should be skipped

	l.Require().NoError(logStorage.Save("test-machine", strings.NewReader("aaaa")), "failed to save log")

	filePath := filepath.Join(tempDir, "test-machine.log")
	checksumPath := filepath.Join(tempDir, "test-machine.log.sha256sum")

	_, err := os.Stat(filePath)
	l.Require().ErrorIs(err, os.ErrNotExist, "log file should not exist, as it was not marked as dirty")

	// mark it dirty and assert that it gets saved

	logStorage.MarkDirty("test-machine")

	l.Require().NoError(logStorage.Save("test-machine", strings.NewReader("aaaa")), "failed to save log")

	_, err = os.Stat(filePath)
	l.Require().NoError(err, "failed to read log file")

	_, err = os.Stat(checksumPath)
	l.Require().NoError(err, "failed to read log file checksum")

	// remove the file, save, and assert that it does not come back, as the dirty flag was cleared

	l.Require().NoError(os.Remove(filePath), "failed to remove log file")
	l.Require().NoError(os.Remove(checksumPath), "failed to remove log file checksum")

	l.Require().NoError(logStorage.Save("test-machine", strings.NewReader("aaaa")), "failed to save log")

	_, err = os.Stat(filePath)
	l.Require().ErrorIs(err, os.ErrNotExist, "log file should not exist, as its dirty flag should have been cleared")

	_, err = os.Stat(checksumPath)
	l.Require().ErrorIs(err, os.ErrNotExist, "log file checksum should not exist, as its dirty flag should have been cleared")
}

// TestSave tests that log storage saves logs on file system with their hash.
func (l *LogStorageSuite) TestSave() {
	for _, tt := range l.tests() {
		l.Run(tt.name, func() {
			logger := zaptest.NewLogger(l.T())

			tempDir := l.T().TempDir()
			logStorage := logstorage.New(tempDir, tt.compressed, logger)

			logStorage.MarkDirty("test-machine")

			l.Require().NoError(logStorage.Save("test-machine", strings.NewReader("aaaa")), "failed to save log")

			fileName := "test-machine.log"
			expectedBytes := []byte("aaaa")

			if tt.compressed {
				fileName = "test-machine.log." + logstorage.CompressionExtension
				expectedBytes = compress(l.T(), expectedBytes)
			}

			fileBytes, err := os.ReadFile(filepath.Join(tempDir, fileName))
			l.Require().NoError(err, "failed to read log file")

			l.Require().Equal(expectedBytes, fileBytes, "log file contents are not equal")

			hashFileName := "test-machine.log.sha256sum"
			if tt.compressed {
				hashFileName = fmt.Sprintf("test-machine.log.%s.sha256sum", logstorage.CompressionExtension)
			}

			fileHashBytes, err := os.ReadFile(filepath.Join(tempDir, hashFileName))
			l.Require().NoError(err, "failed to read log file sha256sum")

			expectedHash := sha256Hex([]byte("aaaa"))
			if tt.compressed {
				expectedHash = sha256Hex(expectedBytes)
			}

			assert.Equal(l.T(), expectedHash, string(fileHashBytes), "log file hash is not equal")
		})
	}
}

// TestLoad tests that log storage loads logs from file system.
func (l *LogStorageSuite) TestLoad() {
	for _, tt := range l.tests() {
		l.Run(tt.name, func() {
			logger := zaptest.NewLogger(l.T())

			tempDir := l.T().TempDir()
			logStorage := logstorage.New(tempDir, tt.compressed, logger)

			logStorage.MarkDirty("test-machine")

			l.Require().NoError(logStorage.Save("test-machine", strings.NewReader("aaaa")), "failed to save log")

			var buffer strings.Builder

			l.Require().NoError(logStorage.Load("test-machine", &buffer))

			l.Assert().Equal("aaaa", buffer.String(), "log file contents are not equal")
		})
	}
}

func (l *LogStorageSuite) TestLoadInvalid() {
	for _, tt := range l.tests() {
		l.Run(tt.name, func() {
			logger := zaptest.NewLogger(l.T())

			tempDir := l.T().TempDir()
			logStorage := logstorage.New(tempDir, tt.compressed, logger)

			logStorage.MarkDirty("test-machine")

			l.Require().NoError(logStorage.Save("test-machine", strings.NewReader("aaaa")), "failed to save log")

			hashFileName := "test-machine.log.sha256sum"
			if tt.compressed {
				hashFileName = fmt.Sprintf("test-machine.log.%s.sha256sum", logstorage.CompressionExtension)
			}

			// corrupt the log file hash
			l.Require().NoError(os.WriteFile(filepath.Join(tempDir, hashFileName), []byte("invalid-hash"), 0o644))

			var buffer strings.Builder

			l.Require().Error(logStorage.Load("test-machine", &buffer))

			l.Assert().NoFileExists(filepath.Join(tempDir, "test-machine.log"), "log file was not removed")
			l.Assert().NoFileExists(filepath.Join(tempDir, "test-machine.log.sha256sum"), "log file was not removed")
		})
	}
}

// TestRemove tests that log storage cal properly remove logs from file system.
func (l *LogStorageSuite) TestRemove() {
	for _, tt := range l.tests() {
		l.Run(tt.name, func() {
			logger := zaptest.NewLogger(l.T())

			tempDir := l.T().TempDir()
			logStorage := logstorage.New(tempDir, tt.compressed, logger)

			l.Require().NoError(logStorage.Save("test-machine", strings.NewReader("aaaa")), "failed to save log")

			l.Require().NoError(logStorage.Remove("test-machine"))

			l.Require().NoFileExists(filepath.Join(tempDir, "test-machine.log"), "log file was not removed")
			l.Require().NoFileExists(filepath.Join(tempDir, "test-machine.log."+logstorage.CompressionExtension), "log file was not removed")
			l.Require().NoFileExists(filepath.Join(tempDir, "test-machine.log.sha256sum"), "log file was not removed")
			l.Require().NoFileExists(filepath.Join(tempDir, fmt.Sprintf("test-machine.log.%s.sha256sum", logstorage.CompressionExtension)), "log file was not removed")
		})
	}
}

type test struct {
	name       string
	compressed bool
}

func (l *LogStorageSuite) tests() []test {
	return []test{
		{"uncompressed", false},
		{"compressed", true},
	}
}

func sha256Hex(data []byte) string {
	sum256Sum := sha256.Sum256(data)

	return hex.EncodeToString(sum256Sum[:])
}

func compress(t *testing.T, data []byte) []byte {
	compressed := &bytes.Buffer{}

	compressingWriter, err := zstd.NewWriter(compressed)
	require.NoError(t, err, "failed to create compressing writer")

	_, err = compressingWriter.Write(data)
	require.NoError(t, err, "failed to write to compressing writer")

	require.NoError(t, compressingWriter.Close(), "failed to close the compressing writer")

	return compressed.Bytes()
}

func TestLogStorageSuite(t *testing.T) {
	suite.Run(t, new(LogStorageSuite))
}
