// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package store

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/etcdbackup"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/etcdbackup/crypt"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/etcdbackup/fstore"
	"github.com/siderolabs/omni/internal/pkg/config"
)

type fileStoreFactory struct {
	store *crypt.Store
	path  string
}

// NewFileStoreStoreFactory returns a new file store factory.
func NewFileStoreStoreFactory(path string) Factory {
	return &fileStoreFactory{
		store: crypt.NewStore(fstore.NewFileStore(path)),
		path:  config.Config.EtcdBackup.GetLocalPath(),
	}
}

func (f *fileStoreFactory) GetStore() (etcdbackup.Store, error) { return f.store, nil }

func (f *fileStoreFactory) Start(ctx context.Context, state state.State, _ *zap.Logger) error {
	if err := setStatus(ctx, state, "local", ""); err != nil {
		return err
	}

	<-ctx.Done()

	return nil
}

func (f *fileStoreFactory) Description() string { return fmt.Sprintf("local store: %s", f.path) }

func (f *fileStoreFactory) SetThroughputs(uint64, uint64) {}
