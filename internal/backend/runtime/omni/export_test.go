// Copyright (c) 2025 Sidero Labs, Inc.
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

func GetEmbeddedEtcdClientWithServer(ctx context.Context, params *config.EtcdParams, logger *zap.Logger, f func(context.Context, *clientv3.Client, func() error) error) error {
	return getEmbeddedEtcdClientWithServerCloser(ctx, params, logger, f)
}

func EtcdElections(ctx context.Context, client *clientv3.Client, electionKey string, logger *zap.Logger, f func(ctx context.Context, client *clientv3.Client) error) error {
	return etcdElections(ctx, client, electionKey, logger, f)
}

func ClusterValidationOptions(st state.State, etcdBackupConfig config.EtcdBackup, embeddedDiscoveryServiceConfig *config.EmbeddedDiscoveryService) []validated.StateOption {
	return clusterValidationOptions(st, etcdBackupConfig, embeddedDiscoveryServiceConfig)
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

func MachineClassValidationOptions(st state.State) []validated.StateOption {
	return machineClassValidationOptions(st)
}

func IdentityValidationOptions(samlConfig config.SAML) []validated.StateOption {
	return identityValidationOptions(samlConfig)
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

func MachineRequestSetValidationOptions(st state.State) []validated.StateOption {
	return machineRequestSetValidationOptions(st)
}

func InfraMachineConfigValidationOptions(st state.State) []validated.StateOption {
	return infraMachineConfigValidationOptions(st)
}

func NodeForceDestroyRequestValidationOptions(st state.State) []validated.StateOption {
	return nodeForceDestroyRequestValidationOptions(st)
}
