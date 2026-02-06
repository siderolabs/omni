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

// NewRotateKubernetesCA creates a new RotateKubernetesCA resource.
func NewRotateKubernetesCA(id resource.ID) *RotateKubernetesCA {
	return typed.NewResource[RotateKubernetesCASpec, RotateKubernetesCAExtension](
		resource.NewMetadata(resources.DefaultNamespace, RotateKubernetesCAType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.RotateKubernetesCASpec{}),
	)
}

// RotateKubernetesCAType is the type of RotateKubernetesCA resource.
//
// tsgen:RotateKubernetesCAType
const RotateKubernetesCAType = resource.Type("RotateKubernetesCAs.omni.sidero.dev")

// RotateKubernetesCA resource describes CA rotation request.
type RotateKubernetesCA = typed.Resource[RotateKubernetesCASpec, RotateKubernetesCAExtension]

// RotateKubernetesCASpec wraps specs.RotateKubernetesCASpec.
type RotateKubernetesCASpec = protobuf.ResourceSpec[specs.RotateKubernetesCASpec, *specs.RotateKubernetesCASpec]

// RotateKubernetesCAExtension providers auxiliary methods for RotateKubernetesCA resource.
type RotateKubernetesCAExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (RotateKubernetesCAExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             RotateKubernetesCAType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
