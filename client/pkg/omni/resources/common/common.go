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

// UserManagedResourceTypes is a list of resource types that are managed by the user.
var UserManagedResourceTypes = []resource.Type{
	authres.IdentityType,
	authres.UserType,
	authres.AccessPolicyType,
	authres.SAMLLabelRuleType,
	siderolink.JoinTokenType,
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
	omni.ImportedClusterSecretsType,
	omni.InfraMachineBMCConfigType,
	omni.InfraMachineConfigType,
	omni.InstallationMediaConfigType,
	omni.NodeForceDestroyRequestType,
	infra.ProviderType,
	omni.RotateTalosCAType,
	omni.RotateKubernetesCAType,
}
