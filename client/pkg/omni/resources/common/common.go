// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package common contains common variables related to resources.
package common

import (
	"github.com/cosi-project/runtime/pkg/resource"

	authres "github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
)

// UserManagedResourceTypes is a list of resource types that are fully managed by the user,
// i.e., the user can perform all CRUD operations on them directly via the state API.
//
// Resources whose lifecycle is partially driven by the user but constrained by the server
// (e.g., ImportedClusterSecrets which only supports create and destroy, Identity and User
// whose mutations must go through ManagementService, or JoinToken whose create must go through
// ManagementService.CreateJoinToken) do not belong here.
var UserManagedResourceTypes = []resource.Type{
	authres.AccessPolicyType,
	authres.SAMLLabelRuleType,
	siderolink.DefaultJoinTokenType,
	siderolink.GRPCTunnelConfigType,
	omni.ClusterType,
	omni.ConfigPatchType,
	omni.EtcdManualBackupType,
	omni.MachineClassType,
	omni.MachineLabelsType,
	omni.MachineSetType,
	omni.MachineSetNodeType,
	omni.EtcdBackupS3ConfType,
	omni.ExtensionsConfigurationType,
	omni.KernelArgsType,
	omni.MachineRequestSetType,
	omni.InfraMachineBMCConfigType,
	omni.InfraMachineConfigType,
	omni.InstallationMediaConfigType,
	omni.NodeForceDestroyRequestType,
	omni.KubernetesManifestGroupType,
	infra.ProviderType,
	omni.RotateTalosCAType,
	omni.RotateKubernetesCAType,
}
