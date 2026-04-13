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

// NewPublicKeyLastActive creates a new PublicKeyLastActive resource.
func NewPublicKeyLastActive(id string) *PublicKeyLastActive {
	return typed.NewResource[PublicKeyLastActiveSpec, PublicKeyLastActiveExtension](
		resource.NewMetadata(resources.MetricsNamespace, PublicKeyLastActiveType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.PublicKeyLastActiveSpec{}),
	)
}

const (
	// PublicKeyLastActiveType is the type of PublicKeyLastActive resource.
	//
	// tsgen:PublicKeyLastActiveType
	PublicKeyLastActiveType = resource.Type("PublicKeyLastActives.omni.sidero.dev")
)

// PublicKeyLastActive resource tracks the last time a specific public key was used to authenticate.
type PublicKeyLastActive = typed.Resource[PublicKeyLastActiveSpec, PublicKeyLastActiveExtension]

// PublicKeyLastActiveSpec wraps specs.PublicKeyLastActiveSpec.
type PublicKeyLastActiveSpec = protobuf.ResourceSpec[specs.PublicKeyLastActiveSpec, *specs.PublicKeyLastActiveSpec]

// PublicKeyLastActiveExtension provides auxiliary methods for PublicKeyLastActive resource.
type PublicKeyLastActiveExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (PublicKeyLastActiveExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             PublicKeyLastActiveType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.MetricsNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
