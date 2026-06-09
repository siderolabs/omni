// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package validations bundles the COSI state validation options applied to user-facing resource
// types in the Omni runtime. The validations enforce structural and semantic rules at the state
// layer, before reconciliation, and apply to every state caller (gRPC API, omnictl, controllers
// with elevated owners). Authorization is handled separately in state_access.go.
package validations

import (
	"slices"

	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/etcdbackup/store"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/validated"
	"github.com/siderolabs/omni/internal/pkg/config"
)

// Options returns the full set of state validation options for all user-facing resource types.
func Options(st state.State, etcdBackupStoreFactory store.Factory, cfg *config.Params) []validated.StateOption {
	return slices.Concat(
		metadataValidationOptions(),
		clusterValidationOptions(st, cfg.EtcdBackup, cfg.Services.EmbeddedDiscoveryService),
		relationLabelsValidationOptions(),
		accessPolicyValidationOptions(),
		roleValidationOptions(),
		machineSetNodeValidationOptions(st),
		machineSetValidationOptions(st, etcdBackupStoreFactory),
		machineClassValidationOptions(st),
		identityValidationOptions(cfg.Auth.Saml),
		accountLimitsValidationOptions(st, cfg.Auth.Limits),
		exposedServiceValidationOptions(),
		configPatchValidationOptions(st),
		etcdManualBackupValidationOptions(),
		samlLabelRuleValidationOptions(),
		s3ConfigValidationOptions(),
		machineRequestSetValidationOptions(st),
		infraMachineConfigValidationOptions(st),
		nodeForceDestroyRequestValidationOptions(st),
		joinTokenValidationOptions(st),
		defaultJoinTokenValidationOptions(st),
		importedClusterSecretValidationOptions(st, cfg.Features.GetEnableClusterImport()),
		infraProviderValidationOptions(st),
		installationMediaConfigValidationOptions(),
		rotateSecretsValidationOptions(st),
		kubernetesManifestsValidationOptions(),
		eulaValidationOptions(st),
	)
}
