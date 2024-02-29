// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package omni

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
)

// NewKubernetesUpgradeManifestStatus creates new LoadBalancer state.
func NewKubernetesUpgradeManifestStatus(ns, id string) *KubernetesUpgradeManifestStatus {
	return typed.NewResource[KubernetesUpgradeManifestStatusSpec, KubernetesUpgradeManifestStatusExtension](
		resource.NewMetadata(ns, KubernetesUpgradeManifestStatusType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.KubernetesUpgradeManifestStatusSpec{}),
	)
}

// KubernetesUpgradeManifestStatusType is a resource type that contains the configuration of a load balancer.
//
// tsgen:KubernetesUpgradeManifestStatusType
const KubernetesUpgradeManifestStatusType = resource.Type("KubernetesUpgradeManifestStatuses.omni.sidero.dev")

// KubernetesUpgradeManifestStatus is a resource type that contains the configuration of a load balancer.
type KubernetesUpgradeManifestStatus = typed.Resource[KubernetesUpgradeManifestStatusSpec, KubernetesUpgradeManifestStatusExtension]

// KubernetesUpgradeManifestStatusSpec wraps specs.KubernetesUpgradeManifestStatusSpec.
type KubernetesUpgradeManifestStatusSpec = protobuf.ResourceSpec[specs.KubernetesUpgradeManifestStatusSpec, *specs.KubernetesUpgradeManifestStatusSpec]

// KubernetesUpgradeManifestStatusExtension providers auxiliary methods for KubernetesUpgradeManifestStatus resource.
type KubernetesUpgradeManifestStatusExtension struct{}

// ResourceDefinition implements typed.ResourceDefinition interface.
func (KubernetesUpgradeManifestStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             KubernetesUpgradeManifestStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
