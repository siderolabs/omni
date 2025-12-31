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

// NewKubernetesUpgradeStatus creates new LoadBalancer state.
func NewKubernetesUpgradeStatus(id string) *KubernetesUpgradeStatus {
	return typed.NewResource[KubernetesUpgradeStatusSpec, KubernetesUpgradeStatusExtension](
		resource.NewMetadata(resources.DefaultNamespace, KubernetesUpgradeStatusType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.KubernetesUpgradeStatusSpec{}),
	)
}

// KubernetesUpgradeStatusType is a resource type that contains the configuration of a load balancer.
//
// tsgen:KubernetesUpgradeStatusType
const KubernetesUpgradeStatusType = resource.Type("KubernetesUpgradeStatuses.omni.sidero.dev")

// KubernetesUpgradeStatus is a resource type that contains the configuration of a load balancer.
type KubernetesUpgradeStatus = typed.Resource[KubernetesUpgradeStatusSpec, KubernetesUpgradeStatusExtension]

// KubernetesUpgradeStatusSpec wraps specs.KubernetesUpgradeStatusSpec.
type KubernetesUpgradeStatusSpec = protobuf.ResourceSpec[specs.KubernetesUpgradeStatusSpec, *specs.KubernetesUpgradeStatusSpec]

// KubernetesUpgradeStatusExtension providers auxiliary methods for KubernetesUpgradeStatus resource.
type KubernetesUpgradeStatusExtension struct{}

// ResourceDefinition implements typed.ResourceDefinition interface.
func (KubernetesUpgradeStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             KubernetesUpgradeStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
