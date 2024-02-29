// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package s3store implements [EtcdBackupStore] which stores backups in a specified S3 bucket.
package s3store

import (
	"context"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"github.com/siderolabs/go-pointer"

	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/etcdbackup"
)

// Store stores etcd backups in a specified S3 bucket.
type Store struct {
	client  *s3.Client
	manager *manager.Uploader
	bucket  string
}

// NewStore initializes [Store].
func NewStore(client *s3.Client, manager *manager.Uploader, bucket string) *Store {
	return &Store{
		client:  client,
		manager: manager,
		bucket:  bucket,
	}
}

// Upload stores the data from [io.Reader] in a file. Implements [Store].
func (s *Store) Upload(ctx context.Context, descr etcdbackup.Description, r io.Reader) error {
	uploader := manager.NewUploader(s.client)

	_, err := uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket: pointer.To(s.bucket),
		Key:    pointer.To(path.Join(descr.ClusterUUID, etcdbackup.CreateSnapshotName(descr.Timestamp))),
		Body:   r,
	})
	if err != nil {
		return fmt.Errorf("failed to upload backup to s3: %w", err)
	}

	return nil
}

// Download downloads the backup with the specified name. Implements [Store].
func (s *Store) Download(ctx context.Context, _ []byte, clusterUUID, snapshotName string) (etcdbackup.BackupData, io.ReadCloser, error) {
	result, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: pointer.To(s.bucket),
		Key:    pointer.To(path.Join(clusterUUID, snapshotName)),
	})
	if err != nil {
		return etcdbackup.BackupData{}, nil, fmt.Errorf("failed to get object: %w", err)
	}

	return etcdbackup.BackupData{}, result.Body, nil
}

// ListBackups returns a list of backups. Implements [EtcdBackupStore].
func (s *Store) ListBackups(ctx context.Context, clusterUUID string) (etcdbackup.InfoIterator, error) {
	result, err := s.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: pointer.To(s.bucket),
		Prefix: pointer.To(fmt.Sprintf("%s/", clusterUUID)),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list backups: %w", err)
	}

	contents := result.Contents

	return func() (etcdbackup.Info, bool, error) {
		for {
			if len(contents) == 0 {
				return etcdbackup.Info{}, false, nil
			}

			content := contents[0]
			contents = contents[1:]

			key := pointer.SafeDeref(content.Key)

			uuidStr, snapshotName, found := strings.Cut(key, "/")
			if !found {
				continue
			}

			_, err := uuid.Parse(uuidStr)
			if err != nil {
				continue
			}

			timestamp, err := etcdbackup.ParseSnapshotName(snapshotName)
			if err != nil {
				return etcdbackup.Info{}, true, err
			}

			return etcdbackup.Info{
				Snapshot:  snapshotName,
				Timestamp: timestamp,
				Reader: func() (io.ReadCloser, error) {
					result, err := s.client.GetObject(ctx, &s3.GetObjectInput{
						Bucket: pointer.To(s.bucket),
						Key:    pointer.To(key),
					})
					if err != nil {
						return nil, fmt.Errorf("failed to get object: %w", err)
					}

					return result.Body, nil
				},
				Size: pointer.SafeDeref(content.Size),
			}, true, nil
		}
	}, nil
}
