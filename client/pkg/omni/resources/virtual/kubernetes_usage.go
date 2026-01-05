// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package virtual

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
)

// NewKubernetesUsage creates new Kubernetes usage resource.
func NewKubernetesUsage(id string) *KubernetesUsage {
	return typed.NewResource[KubernetesUsageSpec, KubernetesUsageExtension](
		resource.NewMetadata(resources.VirtualNamespace, KubernetesUsageType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.KubernetesUsageSpec{}),
	)
}

// KubernetesUsageType is a resource type that contains kubernetes resource usage info.
//
// tsgen:KubernetesUsageType
const KubernetesUsageType = resource.Type("KubernetesUsages.omni.sidero.dev")

// KubernetesUsage is a resource type that contains the state of Kubernetes components in the cluster.
type KubernetesUsage = typed.Resource[KubernetesUsageSpec, KubernetesUsageExtension]

// KubernetesUsageSpec wraps specs.KubernetesUsageSpec.
type KubernetesUsageSpec = protobuf.ResourceSpec[specs.KubernetesUsageSpec, *specs.KubernetesUsageSpec]

// KubernetesUsageExtension providers auxiliary methods for KubernetesUsage resource.
type KubernetesUsageExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (KubernetesUsageExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             KubernetesUsageType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.VirtualNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
