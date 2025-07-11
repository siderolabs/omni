// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package siderolink

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
)

// NewJoinTokenUsage creates a new JoinTokenUsage resource.
func NewJoinTokenUsage(id string) *JoinTokenUsage {
	return typed.NewResource[JoinTokenUsageSpec, JoinTokenUsageExtension](
		resource.NewMetadata(resources.DefaultNamespace, JoinTokenUsageType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.JoinTokenUsageSpec{}),
	)
}

const (
	// JoinTokenUsageType is the type of JoinTokenUsage resource.
	//
	// tsgen:JoinTokenUsageType
	JoinTokenUsageType = resource.Type("JoinTokenUsages.omni.sidero.dev")
)

// JoinTokenUsage keeps the relation between each token and the links using it.
// This resource is identified by the machine ID.
type JoinTokenUsage = typed.Resource[JoinTokenUsageSpec, JoinTokenUsageExtension]

// JoinTokenUsageSpec wraps specs.JoinTokenUsageSpec.
type JoinTokenUsageSpec = protobuf.ResourceSpec[specs.JoinTokenUsageSpec, *specs.JoinTokenUsageSpec]

// JoinTokenUsageExtension providers auxiliary methods for JoinTokenUsage resource.
type JoinTokenUsageExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (JoinTokenUsageExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             JoinTokenUsageType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
