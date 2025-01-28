// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build unix

package fstore

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/etcdbackup"
)

// NewFileStore initializes FileStore.
func NewFileStore(dir string) *FileStore {
	if dir == "" {
		panic(errors.New("dir must be specified"))
	}

	return &FileStore{dir: dir}
}

func uploadFile(dir string, descr etcdbackup.Description, r io.Reader) error {
	err := os.MkdirAll(dir, 0o0755)
	if err != nil {
		return fmt.Errorf("failed to create dir: %w", err)
	}

	fullpath := filepath.Join(dir, etcdbackup.CreateSnapshotName(descr.Timestamp))

	return atomicFileWrite(fullpath, r)
}

func downloadFile(dir string, snapshotName string) (io.ReadCloser, error) {
	fullpath := filepath.Join(dir, snapshotName)

	f, err := os.Open(fullpath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	return f, nil
}

func atomicFileWrite(dst string, r io.Reader) error {
	tempfile := dst + ".tmp"

	tmpFile, err := os.OpenFile(tempfile, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o666)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}

	tempFileCloser := sync.OnceValue(tmpFile.Close)
	tempFileRenamed := false

	defer func() {
		tempFileCloser() //nolint:errcheck

		if !tempFileRenamed {
			os.Remove(tempfile) //nolint:errcheck
		}
	}()

	_, err = io.Copy(tmpFile, r)
	if err != nil {
		return fmt.Errorf("failed to write data: %w", err)
	}

	err = tmpFile.Sync()
	if err != nil {
		return fmt.Errorf("failed to sync data: %w", err)
	}

	err = tempFileCloser()
	if err != nil {
		return fmt.Errorf("failed to close file: %w", err)
	}

	err = os.Rename(tempfile, dst)
	if err != nil {
		return fmt.Errorf("failed to rename file: %w", err)
	}

	tempFileRenamed = true

	return nil
}
