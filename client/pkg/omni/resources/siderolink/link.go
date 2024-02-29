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
)

// NewLink creates new Link state.
func NewLink(ns, id string, spec *specs.SiderolinkSpec) *Link {
	return typed.NewResource[LinkSpec, LinkExtension](
		resource.NewMetadata(ns, LinkType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(spec),
	)
}

// LinkType is the type of Link resource.
//
// tsgen:SiderolinkResourceType
const LinkType = resource.Type("Links.omni.sidero.dev")

// Link resource keeps connected nodes state.
//
// Link resource ID is a machine UUID.
type Link = typed.Resource[LinkSpec, LinkExtension]

// LinkSpec wraps specs.SiderolinkSpec.
type LinkSpec = protobuf.ResourceSpec[specs.SiderolinkSpec, *specs.SiderolinkSpec]

// LinkExtension providers auxiliary methods for Link resource.
type LinkExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (LinkExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             LinkType,
		Aliases:          []resource.Type{},
		DefaultNamespace: Namespace,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Connected",
				JSONPath: "{.connected}",
			},
			{
				Name:     "LastEndpoint",
				JSONPath: "{.lastendpoint}",
			},
		},
	}
}
