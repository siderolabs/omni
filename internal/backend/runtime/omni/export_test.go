// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"

	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/etcdbackup/store"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/validated"
	"github.com/siderolabs/omni/internal/pkg/config"
)

func NewMockState(st state.State) *State {
	return &State{
		defaultState: st,
	}
}

func NewEtcdPersistentState(ctx context.Context, params *config.Params, logger *zap.Logger) (*PersistentState, error) {
	return newEtcdPersistentState(ctx, params, logger)
}

func GetEmbeddedEtcdClientWithServer(params *config.EtcdParams, logger *zap.Logger) (EtcdState, error) {
	return getEmbeddedEtcdState(params, logger)
}

func ClusterValidationOptions(st state.State, etcdBackupConfig config.EtcdBackup, embeddedDiscoveryServiceConfig config.EmbeddedDiscoveryService) []validated.StateOption {
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

func JoinTokenValidationOptions(st state.State) []validated.StateOption {
	return joinTokenValidationOptions(st)
}

func DefaultJoinTokenValidationOptions(st state.State) []validated.StateOption {
	return defaultJoinTokenValidationOptions(st)
}

func ImportedClusterSecretValidationOptions(st state.State, clusterImportEnabled bool) []validated.StateOption {
	return importedClusterSecretValidationOptions(st, clusterImportEnabled)
}

func InfraProviderValidationOptions(st state.State) []validated.StateOption {
	return infraProviderValidationOptions(st)
}

func RotateSecretsValidationOptions(st state.State) []validated.StateOption {
	return rotateSecretsValidationOptions(st)
}

func InstallationMediaConfigValidationOptions() []validated.StateOption {
	return installationMediaConfigValidationOptions()
}

func KubernetesManifestsValidationOptions() []validated.StateOption {
	return kubernetesManifestsValidationOptions()
}
