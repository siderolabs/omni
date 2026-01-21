// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package store

import (
	"context"
	"errors"

	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/etcdbackup"
)

// DisabledStoreFactory is a disabled store factory.
var DisabledStoreFactory Factory = &disabledStoreFactory{}

type disabledStoreFactory struct{}

func (d *disabledStoreFactory) GetStore() (etcdbackup.Store, error) {
	return nil, errors.New("etcd backup store is disabled")
}

func (d *disabledStoreFactory) Start(ctx context.Context, state state.State, _ *zap.Logger) error {
	if err := setStatus(ctx, state, "disabled", ""); err != nil {
		return err
	}

	<-ctx.Done()

	return nil
}

func (d *disabledStoreFactory) Description() string { return "disabled store" }

func (d *disabledStoreFactory) SetThroughputs(uint64, uint64) {}
