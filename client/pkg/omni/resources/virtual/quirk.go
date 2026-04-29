// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package virtual

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
)

// NewQuirks creates a new Quirks resource.
func NewQuirks(id string) *Quirks {
	return typed.NewResource[QuirksSpec, QuirksExtension](
		resource.NewMetadata(resources.VirtualNamespace, QuirksType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.QuirksSpec{}),
	)
}

const (
	// QuirksType is the type of Quirks resource.
	//
	// tsgen:QuirksType
	QuirksType = resource.Type("Quirks.omni.sidero.dev")
)

// Quirks resource describes the current Stripe subscription plan.
type Quirks = typed.Resource[QuirksSpec, QuirksExtension]

// QuirksSpec wraps specs.QuirksSpec.
type QuirksSpec = protobuf.ResourceSpec[specs.QuirksSpec, *specs.QuirksSpec]

// QuirksExtension provides auxiliary methods for Quirks resource.
type QuirksExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (QuirksExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             QuirksType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.VirtualNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
