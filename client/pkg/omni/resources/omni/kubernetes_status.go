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

// NewKubernetesStatus creates new Kubernetes component version/readiness state.
func NewKubernetesStatus(id string) *KubernetesStatus {
	return typed.NewResource[KubernetesStatusSpec, KubernetesStatusExtension](
		resource.NewMetadata(resources.DefaultNamespace, KubernetesStatusType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.KubernetesStatusSpec{}),
	)
}

// KubernetesStatusType is a resource type that contains the state of Kubernetes components in the cluster.
//
// tsgen:KubernetesStatusType
const KubernetesStatusType = resource.Type("KubernetesStatuses.omni.sidero.dev")

// KubernetesStatus is a resource type that contains the state of Kubernetes components in the cluster.
type KubernetesStatus = typed.Resource[KubernetesStatusSpec, KubernetesStatusExtension]

// KubernetesStatusSpec wraps specs.KubernetesStatusSpec.
type KubernetesStatusSpec = protobuf.ResourceSpec[specs.KubernetesStatusSpec, *specs.KubernetesStatusSpec]

// KubernetesStatusExtension providers auxiliary methods for KubernetesStatus resource.
type KubernetesStatusExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (KubernetesStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             KubernetesStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
