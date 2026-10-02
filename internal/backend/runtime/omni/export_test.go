// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/etcdbackup/store"
	"github.com/siderolabs/omni/internal/pkg/config"
)

func NewMockState(st state.State) (*State, error) {
	storeFactory, err := store.NewStoreFactory(config.EtcdBackup{})
	if err != nil {
		return nil, err
	}

	return &State{
		defaultState: st,
		storeFactory: storeFactory,
	}, nil
}

func NewEtcdPersistentState(ctx context.Context, params *config.Params, logger *zap.Logger) (*PersistentState, error) {
	return newEtcdPersistentState(ctx, params, nil, nil, logger)
}

func GetEmbeddedEtcdClientWithServer(params *config.EtcdParams, logger *zap.Logger) (EtcdState, error) {
	return getEmbeddedEtcdState(params, logger)
}

// FilterAccessByType exposes filterAccessByType for tests.
func FilterAccessByType(access state.Access) error {
	return filterAccessByType(access)
}

// CheckForKindAccess exposes checkForKindAccess for tests.
func CheckForKindAccess(ctx context.Context, st state.State, verb state.Verb, kind resource.Kind, labelTerms []resource.LabelTerm) error {
	return checkForKindAccess(ctx, st, verb, kind, labelTerms)
}
