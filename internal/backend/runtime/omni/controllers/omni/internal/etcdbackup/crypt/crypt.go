// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package crypt implements encryption and decryption on top of [etcdbackup.Store].
package crypt

import (
	"context"
	"fmt"
	"io"

	"github.com/siderolabs/omni/client/pkg/panichandler"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/etcdbackup"
)

// Store wraps [Store] and encrypts the data from [io.Reader] before passing it to
// wrapped uploader.
type Store struct {
	wrapped etcdbackup.Store
}

// NewStore initializes [Store].
func NewStore(wrapped etcdbackup.Store) *Store {
	return &Store{wrapped: wrapped}
}

// Upload encrypts the data from [io.Reader] and passes it to wrapped uploader. Implements [Store].
func (c *Store) Upload(ctx context.Context, descr etcdbackup.Description, r io.Reader) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	eg, ctx := panichandler.ErrGroupWithContext(ctx)

	reader, writer := io.Pipe()

	// Close writer if ctx is canceled, so that EncryptEtcdBackup can unblock and return.
	context.AfterFunc(ctx, func() { writer.CloseWithError(ctx.Err()) })

	eg.Go(func() error {
		err := Encrypt(writer, descr.EncryptionData, r)
		if err != nil {
			writer.CloseWithError(err)

			return err
		}

		writer.Close() //nolint:errcheck

		return nil
	})

	eg.Go(func() error { return c.wrapped.Upload(ctx, descr, reader) })

	if err := eg.Wait(); err != nil {
		return fmt.Errorf("failed in wrapped uploader: %w", err)
	}

	return nil
}

// Download downloads the backup for the given cluster UUID and the snapshot name from the wrapped store
// and decrypts it using the given encryption key.
func (c *Store) Download(ctx context.Context, encryptionKey []byte, clusterUUID, snapshotName string) (etcdbackup.BackupData, io.ReadCloser, error) {
	_, encryptedReader, err := c.wrapped.Download(ctx, encryptionKey, clusterUUID, snapshotName)
	if err != nil {
		return etcdbackup.BackupData{}, nil, fmt.Errorf("failed to download backup: %w", err)
	}

	decryptedData, decryptedReader, err := Decrypt(encryptedReader, encryptionKey)
	if err != nil {
		return etcdbackup.BackupData{}, nil, fmt.Errorf("error decrypting backup: %w", err)
	}

	readCloser := struct {
		io.Reader
		io.Closer
	}{
		Reader: decryptedReader,
		Closer: encryptedReader,
	}

	return etcdbackup.BackupData{
		AESCBCEncryptionSecret:    decryptedData.AESCBCEncryptionSecret,
		SecretboxEncryptionSecret: decryptedData.SecretboxEncryptionSecret,
	}, readCloser, nil
}

// ListBackups returns a list of backups. Implements [Store].
func (c *Store) ListBackups(ctx context.Context, uuid string) (etcdbackup.InfoIterator, error) {
	return c.wrapped.ListBackups(ctx, uuid)
}
