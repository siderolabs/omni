// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package auth

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
)

// NewIdentityLastActive creates a new IdentityLastActive resource.
func NewIdentityLastActive(id string) *IdentityLastActive {
	return typed.NewResource[IdentityLastActiveSpec, IdentityLastActiveExtension](
		resource.NewMetadata(resources.MetricsNamespace, IdentityLastActiveType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.IdentityLastActiveSpec{}),
	)
}

const (
	// IdentityLastActiveType is the type of IdentityLastActive resource.
	//
	// tsgen:IdentityLastActiveType
	IdentityLastActiveType = resource.Type("IdentityLastActives.omni.sidero.dev")
)

// IdentityLastActive resource tracks the last time a user or service account was active.
type IdentityLastActive = typed.Resource[IdentityLastActiveSpec, IdentityLastActiveExtension]

// IdentityLastActiveSpec wraps specs.IdentityLastActiveSpec.
type IdentityLastActiveSpec = protobuf.ResourceSpec[specs.IdentityLastActiveSpec, *specs.IdentityLastActiveSpec]

// IdentityLastActiveExtension provides auxiliary methods for IdentityLastActive resource.
type IdentityLastActiveExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (IdentityLastActiveExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             IdentityLastActiveType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.MetricsNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
