// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
)

// NewKubernetesResource creates new cluster config version resource.
func NewKubernetesResource(id resource.ID, spec KubernetesResourceSpec) *KubernetesResource {
	return typed.NewResource[KubernetesResourceSpec, KubernetesResourceExtension](
		resource.NewMetadata(resources.DefaultNamespace, KubernetesResourceType, id, resource.VersionUndefined),
		spec,
	)
}

// KubernetesResourceType is the type of KubernetesResource resource.
//
// tsgen:KubernetesResourceType
const KubernetesResourceType = resource.Type("KubernetesResources.omni.sidero.dev")

// KubernetesResource wraps the Kubernetes Resource.
type KubernetesResource = typed.Resource[KubernetesResourceSpec, KubernetesResourceExtension]

// KubernetesResourceSpec wraps specs.KubernetesResourceSpec.
type KubernetesResourceSpec string

// DeepCopy implements DeepCopyable interface.
func (s KubernetesResourceSpec) DeepCopy() KubernetesResourceSpec {
	return s
}

// KubernetesResourceExtension provides auxiliary methods for KubernetesResource.
type KubernetesResourceExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (KubernetesResourceExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             KubernetesResourceType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
