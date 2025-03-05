// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package fstore implements a file store for etcd backups.
package fstore

import (
	"context"
	"fmt"
	"io"
	"iter"
	"os"
	"path/filepath"

	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/etcdbackup"
)

// FileStore stores etcd backups in a specified directory. It creates a new file for each backup.
// It creates full directory tree if it doesn't exist.
type FileStore struct {
	dir string
}

// Upload stores the data from [io.Reader] in a file. Implements [etcdbackup.Store].
func (store *FileStore) Upload(_ context.Context, descr etcdbackup.Description, r io.Reader) error {
	dir := filepath.Join(store.dir, descr.ClusterUUID)

	return uploadFile(dir, descr, r)
}

// Download returns a reader for the backup file. Implements [etcdbackup.Store].
func (store *FileStore) Download(_ context.Context, _ []byte, clusterUUID, snapshotName string) (etcdbackup.BackupData, io.ReadCloser, error) {
	dir := filepath.Join(store.dir, clusterUUID)

	readCloser, err := downloadFile(dir, snapshotName)

	return etcdbackup.BackupData{}, readCloser, err
}

// ListBackups returns a list of backups. Implements [Store].
func (store *FileStore) ListBackups(_ context.Context, uuid string) (iter.Seq2[etcdbackup.Info, error], error) {
	storeAbsDir, err := filepath.Abs(store.dir)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	backupFiles, err := filepath.Glob(filepath.Join(storeAbsDir, uuid, "*.snapshot"))
	if err != nil {
		return nil, fmt.Errorf("failed to read dir: %w", err)
	}

	return func(yield func(etcdbackup.Info, error) bool) {
		for _, backupFile := range backupFiles {
			stat, err := os.Stat(backupFile)
			if err != nil {
				if !yield(etcdbackup.Info{}, fmt.Errorf("failed to get file info: %w", err)) {
					return
				}

				continue
			}

			timestamp, err := etcdbackup.ParseSnapshotName(stat.Name())
			if err != nil {
				if !yield(etcdbackup.Info{}, fmt.Errorf("failed to parse snapshot name: %w", err)) {
					return
				}

				continue
			}

			if !yield(etcdbackup.Info{
				Snapshot:  stat.Name(),
				Timestamp: timestamp,
				Reader:    func() (io.ReadCloser, error) { return os.Open(backupFile) },
				Size:      stat.Size(),
			}, nil) {
				return
			}
		}
	}, nil
}
