// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package fstore

import (
	"errors"
	"io"

	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/etcdbackup"
)

// NewFileStore initializes FileStore.
func NewFileStore(dir string) *FileStore {
	panic("file store is not supported on windows")
}

func uploadFile(dir string, descr etcdbackup.Description, r io.Reader) error {
	return errors.New("file store is not supported on windows")
}

func downloadFile(dir string, snapshotName string) (io.ReadCloser, error) {
	return nil, errors.New("file store is not supported on windows")
}
