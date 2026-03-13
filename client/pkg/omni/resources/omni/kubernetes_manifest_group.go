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

// NewKubernetesManifestGroup creates new kubernetes manifest resource.
func NewKubernetesManifestGroup(id resource.ID) *KubernetesManifestGroup {
	return typed.NewResource[KubernetesManifestGroupSpec, KubernetesManifestGroupExtension](
		resource.NewMetadata(resources.DefaultNamespace, KubernetesManifestGroupType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.KubernetesManifestGroupSpec{}),
	)
}

const (
	// KubernetesManifestGroupType is the type of the KubernetesManifestGroup resource.
	// tsgen:KubernetesManifestGroupType
	KubernetesManifestGroupType = resource.Type("KubernetesManifestGroups.omni.sidero.dev")
)

// KubernetesManifestGroup is applied by Omni on a workload cluster.
// It can have any number of kubernetes manifests inside.
type KubernetesManifestGroup = typed.Resource[KubernetesManifestGroupSpec, KubernetesManifestGroupExtension]

// KubernetesManifestGroupSpec wraps specs.KubernetesManifestGroupSpec.
type KubernetesManifestGroupSpec = protobuf.ResourceSpec[specs.KubernetesManifestGroupSpec, *specs.KubernetesManifestGroupSpec]

// KubernetesManifestGroupExtension provides auxiliary methods for KubernetesManifestGroup resource.
type KubernetesManifestGroupExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (KubernetesManifestGroupExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             KubernetesManifestGroupType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
