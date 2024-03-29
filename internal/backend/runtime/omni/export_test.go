// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/etcdbackup/store"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/validated"
	"github.com/siderolabs/omni/internal/pkg/config"
)

func BuildEtcdPersistentState(ctx context.Context, params *config.Params, logger *zap.Logger, f func(context.Context, namespaced.StateBuilder) error) error {
	return buildEtcdPersistentState(ctx, params, logger, f)
}

func GetEmbeddedEtcdClient(ctx context.Context, params *config.EtcdParams, logger *zap.Logger, f func(context.Context, *clientv3.Client) error) error {
	return getEmbeddedEtcdClient(ctx, params, logger, f)
}

func EtcdElections(ctx context.Context, client *clientv3.Client, electionKey string, logger *zap.Logger, f func(ctx context.Context, client *clientv3.Client) error) error {
	return etcdElections(ctx, client, electionKey, logger, f)
}

func ClusterVersionValidationOptions(st state.State) []validated.StateOption {
	return clusterValidationOptions(st)
}

func RelationLabelsValidationOptions() []validated.StateOption {
	return relationLabelsValidationOptions()
}

func MachineSetValidationOptions(st state.State, etcdBackupStoreFactory store.Factory) []validated.StateOption {
	return machineSetValidationOptions(st, etcdBackupStoreFactory)
}

func MachineSetNodeValidationOptions(st state.State) []validated.StateOption {
	return machineSetNodeValidationOptions(st)
}

func IdentityValidationOptions() []validated.StateOption {
	return identityValidationOptions()
}

func ExposedServiceValidationOptions() []validated.StateOption {
	return exposedServiceValidationOptions()
}

func ConfigPatchValidationOptions(st state.State) []validated.StateOption {
	return configPatchValidationOptions(st)
}

func EtcdManualBackupValidationOptions() []validated.StateOption {
	return etcdManualBackupValidationOptions()
}

func SAMLLabelRuleValidationOptions() []validated.StateOption {
	return samlLabelRuleValidationOptions()
}

func S3ConfigValidationOptions() []validated.StateOption {
	return s3ConfigValidationOptions()
}

func SchematicConfigurationValidationOptions() []validated.StateOption {
	return schematicConfigurationValidationOptions()
}
