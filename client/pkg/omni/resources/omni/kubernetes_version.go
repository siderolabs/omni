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

// KubernetesVersionType is the type of KubernetesVersions resource.
//
// tsgen:KubernetesVersionType
const KubernetesVersionType = resource.Type("KubernetesVersions.omni.sidero.dev")

// KubernetesVersion represents discovered Kubernetes versions to be installed.
type KubernetesVersion = typed.Resource[KubernetesVersionSpec, KubernetesVersionExtension]

// NewKubernetesVersion creates new KubernetesVersion resource.
func NewKubernetesVersion(ns resource.Namespace, id resource.ID) *KubernetesVersion {
	return typed.NewResource[KubernetesVersionSpec, KubernetesVersionExtension](
		resource.NewMetadata(ns, KubernetesVersionType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(
			&specs.KubernetesVersionSpec{},
		),
	)
}

// KubernetesVersionExtension provides auxiliary methods for KubernetesVersion.
type KubernetesVersionExtension struct{}

// KubernetesVersionSpec wraps specs.KubernetesVersionSpec.
type KubernetesVersionSpec = protobuf.ResourceSpec[specs.KubernetesVersionSpec, *specs.KubernetesVersionSpec]

// ResourceDefinition implements [typed.Extension] interface.
func (KubernetesVersionExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             KubernetesVersionType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
