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

// NewJoinToken creates a new JoinToken resource.
func NewJoinToken(id string) *JoinToken {
	return typed.NewResource[JoinTokenSpec, JoinTokenExtension](
		resource.NewMetadata(resources.DefaultNamespace, JoinTokenType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.JoinTokenSpec{}),
	)
}

const (
	// JoinTokenType is the type of JoinToken resource.
	//
	// tsgen:JoinTokenType
	JoinTokenType = resource.Type("JoinTokens.omni.sidero.dev")
)

// JoinToken resource keeps the available join tokens that Talos nodes can use for joining Omni.
type JoinToken = typed.Resource[JoinTokenSpec, JoinTokenExtension]

// JoinTokenSpec wraps specs.JoinTokenSpec.
type JoinTokenSpec = protobuf.ResourceSpec[specs.JoinTokenSpec, *specs.JoinTokenSpec]

// JoinTokenExtension providers auxiliary methods for JoinToken resource.
type JoinTokenExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (JoinTokenExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             JoinTokenType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
