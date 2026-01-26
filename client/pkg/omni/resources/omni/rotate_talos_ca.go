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

// NewRotateTalosCA creates a new RotateTalosCA resource.
func NewRotateTalosCA(id resource.ID) *RotateTalosCA {
	return typed.NewResource[RotateTalosCASpec, RotateTalosCAExtension](
		resource.NewMetadata(resources.DefaultNamespace, RotateTalosCAType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.RotateTalosCASpec{}),
	)
}

// RotateTalosCAType is the type of RotateTalosCA resource.
//
// tsgen:RotateTalosCAType
const RotateTalosCAType = resource.Type("RotateTalosCAs.omni.sidero.dev")

// RotateTalosCA resource describes CA rotation request.
type RotateTalosCA = typed.Resource[RotateTalosCASpec, RotateTalosCAExtension]

// RotateTalosCASpec wraps specs.RotateTalosCASpec.
type RotateTalosCASpec = protobuf.ResourceSpec[specs.RotateTalosCASpec, *specs.RotateTalosCASpec]

// RotateTalosCAExtension providers auxiliary methods for RotateTalosCA resource.
type RotateTalosCAExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (RotateTalosCAExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             RotateTalosCAType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
