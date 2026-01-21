// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package fstore_test

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/siderolabs/gen/xtesting/check"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/etcdbackup/fstore"
)

const dir = "testdata"

var filePath = filepath.Join(dir, "test.txt")

func TestAtomicWriteFile(t *testing.T) {
	tests := []struct { //nolint:govet
		name         string
		r            io.Reader
		errCheck     check.Check
		continuation func(t *testing.T)
	}{
		{
			name:     "simple file",
			r:        strings.NewReader("Hello World"),
			errCheck: check.NoError(),
			continuation: func(t *testing.T) {
				file, err := os.ReadFile(filePath)
				require.NoError(t, err)

				require.Equal(t, "Hello World", string(file))

				entries, err := os.ReadDir(dir)
				require.NoError(t, err)
				require.Len(t, entries, 1)
			},
		},
		{
			name:         "error reader",
			r:            &errReader{},
			errCheck:     check.ErrorContains("test error"),
			continuation: func(*testing.T) {},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.NoError(t, os.Mkdir(dir, 0o755))

			t.Cleanup(func() { require.NoError(t, os.RemoveAll(dir)) })

			test.errCheck(t, fstore.AtomicFileWrite(filePath, test.r))
			test.continuation(t)
		})
	}
}

type errReader struct{ err error }

func (r *errReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	if r.err == nil {
		r.err = errors.New("test error")

		return copy(p, "Hello World"), nil
	}

	return 0, r.err
}
