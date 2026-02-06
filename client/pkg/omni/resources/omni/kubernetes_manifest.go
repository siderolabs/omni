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

// NewKubernetesManifest creates new kubernetes manifest resource.
func NewKubernetesManifest(id resource.ID) *KubernetesManifest {
	return typed.NewResource[KubernetesManifestSpec, KubernetesManifestExtension](
		resource.NewMetadata(resources.DefaultNamespace, KubernetesManifestType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.KubernetesManifestSpec{}),
	)
}

const (
	// KubernetesManifestType is the type of the KubernetesManifest resource.
	// tsgen:KubernetesManifestType
	KubernetesManifestType = resource.Type("KubernetesManifests.omni.sidero.dev")
)

// KubernetesManifest is applied by Omni on a workload cluster.
type KubernetesManifest = typed.Resource[KubernetesManifestSpec, KubernetesManifestExtension]

// KubernetesManifestSpec wraps specs.KubernetesManifestSpec.
type KubernetesManifestSpec = protobuf.ResourceSpec[specs.KubernetesManifestSpec, *specs.KubernetesManifestSpec]

// KubernetesManifestExtension provides auxiliary methods for KubernetesManifest resource.
type KubernetesManifestExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (KubernetesManifestExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             KubernetesManifestType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
