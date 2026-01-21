// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package etcdbackup implements is a public part of internal/etcdbackup package.
package etcdbackup

import (
	"context"
	"fmt"
	"io"
	"iter"
	"strconv"
	"strings"
	"time"
)

// Lister is an interface that is used to list etcd backups.
type Lister interface {
	ListBackups(ctx context.Context, clusterUUID string) (iter.Seq2[Info, error], error)
}

// Store is an interface that is used to upload etcd backups.
type Store interface {
	Lister
	Upload(ctx context.Context, descr Description, r io.Reader) error
	Download(ctx context.Context, encryptionKey []byte, clusterUUID, snapshotName string) (BackupData, io.ReadCloser, error)
}

// Description is a data that is used to describe backup.
type Description struct {
	Timestamp      time.Time
	ClusterUUID    string
	ClusterName    string
	EncryptionData EncryptionData
}

// Info describes existing etcd backup that we got from external resources such as S3 or filesystem.
type Info struct {
	Timestamp time.Time
	Reader    func() (io.ReadCloser, error)
	Snapshot  string
	Size      int64
}

// BackupData contains additional data related to the etcd backup.
type BackupData struct {
	AESCBCEncryptionSecret    string
	SecretboxEncryptionSecret string
}

// EncryptionData contains data required to encrypt etcd backup.
type EncryptionData struct {
	AESCBCEncryptionSecret    string
	SecretboxEncryptionSecret string
	EncryptionKey             []byte
}

// CreateSnapshotName creates a name for a snapshot.
func CreateSnapshotName(timestamp time.Time) string {
	return fmt.Sprintf("%016X.snapshot", uint64(-timestamp.Unix()))
}

// ParseSnapshotName parses a snapshot name. Returns backup timestamp and expiration.
func ParseSnapshotName(name string) (time.Time, error) {
	reverseTSRaw, found := strings.CutSuffix(name, ".snapshot")
	if !found {
		return time.Time{}, fmt.Errorf("failed to parse snapshot %q, invalid suffix", name)
	}

	reverseTS, err := strconv.ParseUint(reverseTSRaw, 16, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse snapshot %q : %w", name, err)
	}

	return time.Unix(int64(-reverseTS), 0), nil
}
