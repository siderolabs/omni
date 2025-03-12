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

// NewDefaultJoinToken creates a new DefaultJoinToken resource.
func NewDefaultJoinToken() *DefaultJoinToken {
	return typed.NewResource[DefaultJoinTokenSpec, DefaultJoinTokenExtension](
		resource.NewMetadata(resources.DefaultNamespace, DefaultJoinTokenType, DefaultJoinTokenID, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.DefaultJoinTokenSpec{}),
	)
}

const (
	// DefaultJoinTokenID is the ID of the DefaultJoinToken resource.
	// tsgen:DefaultJoinTokenID
	DefaultJoinTokenID = resource.ID("default")

	// DefaultJoinTokenType is the type of DefaultJoinToken resource.
	//
	// tsgen:DefaultJoinTokenType
	DefaultJoinTokenType = resource.Type("DefaultJoinTokens.omni.sidero.dev")
)

// DefaultJoinToken is the user managed resource that marks some join token as the default one.
type DefaultJoinToken = typed.Resource[DefaultJoinTokenSpec, DefaultJoinTokenExtension]

// DefaultJoinTokenSpec wraps specs.DefaultJoinTokenSpec.
type DefaultJoinTokenSpec = protobuf.ResourceSpec[specs.DefaultJoinTokenSpec, *specs.DefaultJoinTokenSpec]

// DefaultJoinTokenExtension providers auxiliary methods for DefaultJoinToken resource.
type DefaultJoinTokenExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (DefaultJoinTokenExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             DefaultJoinTokenType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
