// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package validations

import (
	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/etcdbackup/store"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/validated"
	"github.com/siderolabs/omni/internal/pkg/config"
)

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

func InstallationMediaConfigValidationOptions(st state.State) []validated.StateOption {
	return installationMediaConfigValidationOptions(st)
}

func AccountLimitsValidationOptions(st state.State, limits config.AuthLimits) []validated.StateOption {
	return accountLimitsValidationOptions(st, limits)
}

func KubernetesManifestsValidationOptions() []validated.StateOption {
	return kubernetesManifestsValidationOptions()
}

func EulaValidationOptions(st state.State) []validated.StateOption {
	return eulaValidationOptions(st)
}

func KernelArgsValidationOptions() []validated.StateOption {
	return kernelArgsValidationOptions()
}

func MetadataValidationOptions() []validated.StateOption {
	return metadataValidationOptions()
}

func KubernetesHealthCheckValidationOptions() []validated.StateOption {
	return kubernetesHealthcheckValidationOptions()
}
