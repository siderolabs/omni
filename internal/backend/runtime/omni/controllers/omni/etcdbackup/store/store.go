// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package store implements a factory for etcd backup stores.
package store

import (
	"context"
	"fmt"
	"sync"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/etcdbackup"
	"github.com/siderolabs/omni/internal/pkg/config"
)

// Factory is a factory for etcd backup stores.
type Factory interface {
	GetStore() (etcdbackup.Store, error)
	Start(context.Context, state.State, *zap.Logger) error
	Description() string
}

// NewStoreFactory returns a new store factory.
func NewStoreFactory() (FactoryWithMetrics, error) {
	return newEtcdBackupStoreFactory()
}

var newEtcdBackupStoreFactory = sync.OnceValues(func() (FactoryWithMetrics, error) {
	storageType, err := config.Config.EtcdBackup.GetStorageType()
	if err != nil {
		return nil, err
	}

	var result Factory

	switch storageType {
	case config.EtcdBackupTypeS3:
		result = NewS3StoreFactory()
	case config.EtcdBackupTypeFS:
		result = NewFileStoreStoreFactory(config.Config.EtcdBackup.LocalPath)
	case config.EtcdBackupTypeNone:
		result = DisabledStoreFactory
	default:
		return nil, fmt.Errorf("unknown storage type: %q", storageType)
	}

	return newFactoryWithMetrics(result), nil
})

func setStatus(ctx context.Context, st state.State, confName, errString string) error {
	status := omni.NewEtcdBackupStoreStatus()
	status.TypedSpec().Value.ConfigurationName = confName
	status.TypedSpec().Value.ConfigurationError = errString

	if err := st.Create(ctx, status); err != nil {
		return fmt.Errorf("failed to create etcd backup overall status: %w", err)
	}

	return nil
}

func updateS3Status(ctx context.Context, st state.State, errString string) error {
	if _, err := safe.StateUpdateWithConflicts(ctx, st, omni.NewEtcdBackupStoreStatus().Metadata(), func(result *omni.EtcdBackupStoreStatus) error {
		result.TypedSpec().Value.ConfigurationError = errString

		return nil
	}); err != nil {
		return fmt.Errorf("failed to update etcd backup overall status: %w", err)
	}

	return nil
}
