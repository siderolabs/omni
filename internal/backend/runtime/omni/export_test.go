// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"go.etcd.io/etcd/server/v3/embed"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/etcdbackup/store"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/validated"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/virtual"
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

// EmbeddedEtcdServer returns the embedded etcd server of the state.
func EmbeddedEtcdServer(st EtcdState) *embed.Etcd {
	return st.(*embeddedEtcd).embeddedServer //nolint:errcheck,forcetypeassert
}

// FilterAccessByType exposes filterAccessByType for tests.
func FilterAccessByType(access state.Access) error {
	return filterAccessByType(access)
}

// CheckForKindAccess exposes checkForKindAccess for tests.
func CheckForKindAccess(ctx context.Context, st state.State, verb state.Verb, kind resource.Kind, labelTerms []resource.LabelTerm) error {
	return checkForKindAccess(ctx, st, verb, kind, labelTerms)
}

// AuthorizationValidationOptions exposes authorizationValidationOptions for tests.
func AuthorizationValidationOptions(st state.State) []validated.StateOption {
	return authorizationValidationOptions(st)
}

// NewAuthorizedState returns the production composition of the namespaced state over in-memory storage, with only the
// authorization checks on top. The second state is the same composition without the authorization checks.
func NewAuthorizedState(apiURL string) (authorized, unauthorized state.State, err error) {
	storeFactory, err := store.NewStoreFactory(config.EtcdBackup{})
	if err != nil {
		return nil, nil, err
	}

	defaultCoreState := inmem.NewState()
	virtualState := virtual.NewState(state.WrapCore(defaultCoreState), nil, apiURL, "", config.Registries{}, config.Support{})

	unauthorized = state.WrapCore(newNamespacedState(defaultCoreState, inmem.NewState(), virtualState, zap.NewNop(), storeFactory))

	return state.WrapCore(validated.NewState(unauthorized, authorizationValidationOptions(unauthorized)...)), unauthorized, nil
}
