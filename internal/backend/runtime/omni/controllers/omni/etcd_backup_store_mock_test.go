// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"iter"
	"sync"

	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/etcdbackup"
)

type mockStoreFactory struct {
	store etcdbackup.Store
}

func (m *mockStoreFactory) SetThroughputs(uint64, uint64) {}

func (m *mockStoreFactory) GetStore() (etcdbackup.Store, error) {
	return m.store, nil
}

func (m *mockStoreFactory) Start(context.Context, state.State, *zap.Logger) error { return nil }

func (m *mockStoreFactory) Description() string { return "mock-store" }

type mockEtcdBackupStore struct {
	logger      *zap.Logger
	descs       []etcdbackup.Description
	backupDatas []etcdbackup.BackupData
	backups     []etcdbackup.Info
	listCalls   []string
	mu          sync.Mutex
}

func (m *mockEtcdBackupStore) getListCalls() []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.listCalls
}

func (m *mockEtcdBackupStore) getBackups() []etcdbackup.Info {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.backups
}

func (m *mockEtcdBackupStore) ListBackups(_ context.Context, clusterUUID string) (iter.Seq2[etcdbackup.Info, error], error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.logger != nil {
		m.logger.Info("mock list backups", zap.String("uuid", clusterUUID), zap.Any("calls", m.listCalls))
	}

	m.listCalls = append(m.listCalls, clusterUUID)

	return func(yield func(etcdbackup.Info, error) bool) {
		for i, b := range m.backups {
			desc := m.descs[i]
			if desc.ClusterUUID != clusterUUID {
				continue
			}

			if !yield(b, nil) {
				return
			}
		}
	}, nil
}

func (m *mockEtcdBackupStore) Upload(ctx context.Context, desc etcdbackup.Description, rdr io.Reader) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.logger != nil {
		m.logger.Info("mock backup upload", zap.Any("desc", desc))
	}

	backupData := etcdbackup.BackupData{
		AESCBCEncryptionSecret:    desc.EncryptionData.AESCBCEncryptionSecret,
		SecretboxEncryptionSecret: desc.EncryptionData.SecretboxEncryptionSecret,
	}

	data, err := io.ReadAll(rdr)
	if err != nil {
		return err
	}

	m.descs = append(m.descs, desc)
	m.backupDatas = append(m.backupDatas, backupData)
	m.backups = append(m.backups, etcdbackup.Info{
		Timestamp: desc.Timestamp,
		Reader: func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(data)), nil
		},
		Snapshot: etcdbackup.CreateSnapshotName(desc.Timestamp),
		Size:     int64(len(data)),
	})

	return nil
}

func (m *mockEtcdBackupStore) Download(ctx context.Context, _ []byte, clusterUUID, snapshotName string) (etcdbackup.BackupData, io.ReadCloser, error) {
	if ctx.Err() != nil {
		return etcdbackup.BackupData{}, nil, ctx.Err()
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	idx := -1

	for i, desc := range m.descs {
		if desc.ClusterUUID == clusterUUID {
			backup := m.backups[i]
			if backup.Snapshot == snapshotName {
				idx = i

				break
			}
		}
	}

	if idx == -1 {
		return etcdbackup.BackupData{}, nil, fmt.Errorf("not found: %s/%s", clusterUUID, snapshotName)
	}

	backupData := m.backupDatas[idx]
	backup := m.backups[idx]

	rdr, err := backup.Reader()
	if err != nil {
		return etcdbackup.BackupData{}, nil, err
	}

	return backupData, rdr, nil
}
