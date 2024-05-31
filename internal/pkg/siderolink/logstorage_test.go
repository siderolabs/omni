// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package siderolink_test

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/omni/internal/pkg/siderolink"
)

type LogStorageSuite struct {
	suite.Suite
}

// TestSave tests that log storage saves logs on file system with their hash.
func (l *LogStorageSuite) TestSave() {
	tempDir := l.T().TempDir()
	logStorage := siderolink.NewLogStorage(tempDir)

	l.Require().NoError(logStorage.Save("test-machine", strings.NewReader("aaaa")), "failed to save log")

	fileBytes, err := os.ReadFile(filepath.Join(tempDir, "test-machine.log"))
	l.Require().NoError(err, "failed to read log file")

	l.Require().Equal("aaaa", string(fileBytes), "log file contents are not equal")

	fileHashBytes, err := os.ReadFile(filepath.Join(tempDir, "test-machine.log.sha256sum"))
	l.Require().NoError(err, "failed to read log file sha256sum")

	expectedHash := sha256Hex([]byte("aaaa"))

	assert.Equal(l.T(), expectedHash, string(fileHashBytes), "log file hash is not equal")
}

// TestLoad tests that log storage loads logs from file system.
func (l *LogStorageSuite) TestLoad() {
	tempDir := l.T().TempDir()
	logStorage := siderolink.NewLogStorage(tempDir)

	l.Require().NoError(logStorage.Save("test-machine", strings.NewReader("aaaa")), "failed to save log")

	var buffer strings.Builder

	l.Require().NoError(logStorage.Load("test-machine", &buffer))

	l.Assert().Equal("aaaa", buffer.String(), "log file contents are not equal")
}

func (l *LogStorageSuite) TestLoadInvalid() {
	tempDir := l.T().TempDir()
	logStorage := siderolink.NewLogStorage(tempDir)

	l.Require().NoError(logStorage.Save("test-machine", strings.NewReader("aaaa")), "failed to save log")

	// corrupt the log file hash
	l.Require().NoError(os.WriteFile(filepath.Join(tempDir, "test-machine.log.sha256sum"), []byte("invalid-hash"), 0o644))

	var buffer strings.Builder

	l.Require().Error(logStorage.Load("test-machine", &buffer))

	l.Assert().NoFileExists(filepath.Join(tempDir, "test-machine.log"), "log file was not removed")
	l.Assert().NoFileExists(filepath.Join(tempDir, "test-machine.log.sha256sum"), "log file was not removed")
}

// TestRemove tests that log storage cal properly remove logs from file system.
func (l *LogStorageSuite) TestRemove() {
	tempDir := l.T().TempDir()
	logStorage := siderolink.NewLogStorage(tempDir)

	l.Require().NoError(logStorage.Save("test-machine", strings.NewReader("aaaa")), "failed to save log")

	l.Require().NoError(logStorage.Remove("test-machine"))

	l.Require().NoFileExists(filepath.Join(tempDir, "test-machine.log"), "log file was not removed")
	l.Require().NoFileExists(filepath.Join(tempDir, "test-machine.log.sha256sum"), "log file was not removed")
}

func sha256Hex(data []byte) string {
	sum256Sum := sha256.Sum256(data)

	return hex.EncodeToString(sum256Sum[:])
}

func TestLogStorageSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(LogStorageSuite))
}
