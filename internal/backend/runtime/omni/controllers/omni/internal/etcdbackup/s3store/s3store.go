// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package s3store implements [EtcdBackupStore] which stores backups in a specified S3 bucket.
package s3store

import (
	"context"
	"fmt"
	"io"
	"iter"
	"path"
	"strings"

	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"github.com/siderolabs/go-pointer"
	"golang.org/x/time/rate"

	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/etcdbackup"
)

// Store stores etcd backups in a specified S3 bucket.
type Store struct {
	client       *s3.Client
	downloadRate *rate.Limiter
	uploadRate   *rate.Limiter
	bucket       string
}

// NewStore initializes [Store].
func NewStore(client *s3.Client, bucket string, upRate, downRate int64) *Store {
	upLimit := rate.Inf
	if upRate > 0 {
		upLimit = rate.Limit(upRate)
	}

	downLimit := rate.Inf
	if downRate > 0 {
		downLimit = rate.Limit(downRate)
	}

	return &Store{
		client:       client,
		downloadRate: rate.NewLimiter(upLimit, int(upRate)),
		uploadRate:   rate.NewLimiter(downLimit, int(downRate)),
		bucket:       bucket,
	}
}

// Upload stores the data from [io.Reader] in a file. Implements [Store].
func (s *Store) Upload(ctx context.Context, descr etcdbackup.Description, r io.Reader) error {
	uploader := manager.NewUploader(s.client)

	_, err := uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket: pointer.To(s.bucket),
		Key:    pointer.To(path.Join(descr.ClusterUUID, etcdbackup.CreateSnapshotName(descr.Timestamp))),
		Body:   newReaderLimiter(io.NopCloser(r), s.uploadRate),
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

	return etcdbackup.BackupData{}, newReaderLimiter(result.Body, s.downloadRate), nil
}

// ListBackups returns a list of backups. Implements [EtcdBackupStore].
func (s *Store) ListBackups(ctx context.Context, clusterUUID string) (iter.Seq2[etcdbackup.Info, error], error) {
	result, err := s.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: pointer.To(s.bucket),
		Prefix: pointer.To(fmt.Sprintf("%s/", clusterUUID)),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list backups: %w", err)
	}

	contents := result.Contents

	return func(yield func(etcdbackup.Info, error) bool) {
		for _, content := range contents {
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
				if !yield(etcdbackup.Info{}, fmt.Errorf("failed to parse snapshot name: %w", err)) {
					return
				}

				continue
			}

			if !yield(etcdbackup.Info{
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
			}, nil) {
				return
			}
		}
	}, nil
}

type readerLimiter struct {
	rdr io.ReadCloser
	l   *rate.Limiter
}

func newReaderLimiter(rdr io.ReadCloser, l *rate.Limiter) *readerLimiter {
	return &readerLimiter{rdr: rdr, l: l}
}

func (r *readerLimiter) Read(p []byte) (int, error) {
	burst := r.l.Burst()
	if burst == 0 {
		return r.rdr.Read(p)
	}

	readInto := p[:min(burst, len(p))]

	err := r.l.WaitN(context.Background(), len(readInto))
	if err != nil {
		return 0, err
	}

	n, err := r.rdr.Read(readInto)
	if err != nil {
		return n, err
	}

	return n, nil
}

func (r *readerLimiter) Close() error {
	return r.rdr.Close()
}
