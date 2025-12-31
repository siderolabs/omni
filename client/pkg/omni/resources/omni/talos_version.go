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

// NewTalosVersion creates new cluster resource.
func NewTalosVersion(id resource.ID) *TalosVersion {
	return typed.NewResource[TalosVersionSpec, TalosVersionExtension](
		resource.NewMetadata(resources.DefaultNamespace, TalosVersionType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.TalosVersionSpec{}),
	)
}

const (
	// TalosVersionType is the type of the TalosVersion resource.
	// tsgen:TalosVersionType
	TalosVersionType = resource.Type("TalosVersions.omni.sidero.dev")
)

// TalosVersion describes available Talos version.
type TalosVersion = typed.Resource[TalosVersionSpec, TalosVersionExtension]

// TalosVersionSpec wraps specs.TalosVersionSpec.
type TalosVersionSpec = protobuf.ResourceSpec[specs.TalosVersionSpec, *specs.TalosVersionSpec]

// TalosVersionExtension provides auxiliary methods for TalosVersion resource.
type TalosVersionExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (TalosVersionExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             TalosVersionType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Kubernetes Versions",
				JSONPath: "{.compatiblekubernetesversions}",
			},
		},
	}
}
