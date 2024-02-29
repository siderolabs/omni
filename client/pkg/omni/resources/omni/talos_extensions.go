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

// NewTalosExtensions creates new Kubernetes component version/readiness state.
func NewTalosExtensions(ns, id string) *TalosExtensions {
	return typed.NewResource[TalosExtensionsSpec, TalosExtensionsExtension](
		resource.NewMetadata(ns, TalosExtensionsType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.TalosExtensionsSpec{}),
	)
}

// TalosExtensionsType is resource type that contains all available extensions for a Talos version.
//
// tsgen:TalosExtensionsType
const TalosExtensionsType = resource.Type("TalosExtensions.omni.sidero.dev")

// TalosExtensions is resource type that contains all available extensions for a Talos version.
type TalosExtensions = typed.Resource[TalosExtensionsSpec, TalosExtensionsExtension]

// TalosExtensionsSpec wraps specs.TalosExtensionsSpec.
type TalosExtensionsSpec = protobuf.ResourceSpec[specs.TalosExtensionsSpec, *specs.TalosExtensionsSpec]

// TalosExtensionsExtension providers auxiliary methods for TalosExtensions resource.
type TalosExtensionsExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (TalosExtensionsExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             TalosExtensionsType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
