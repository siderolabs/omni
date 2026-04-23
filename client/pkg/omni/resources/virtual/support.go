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

// SupportID is the default and the only allowed ID for Support resource.
//
// tsgen:SupportID
const SupportID = "support"

// NewSupport creates a new Support resource.
func NewSupport() *Support {
	return typed.NewResource[SupportSpec, SupportExtension](
		resource.NewMetadata(resources.VirtualNamespace, SupportType, SupportID, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.SupportSpec{}),
	)
}

const (
	// SupportType is the type of Support resource.
	//
	// tsgen:SupportType
	SupportType = resource.Type("Supports.omni.sidero.dev")
)

// Support resource describes the current Stripe subscription plan.
type Support = typed.Resource[SupportSpec, SupportExtension]

// SupportSpec wraps specs.SupportSpec.
type SupportSpec = protobuf.ResourceSpec[specs.SupportSpec, *specs.SupportSpec]

// SupportExtension provides auxiliary methods for Support resource.
type SupportExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (SupportExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             SupportType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.VirtualNamespace,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "SupportEnabled",
				JSONPath: "{.supportenabled}",
			},
		},
	}
}
