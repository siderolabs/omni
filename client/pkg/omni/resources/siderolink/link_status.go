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

// NewLinkStatus creates new LinkStatus state.
func NewLinkStatus(res resource.Resource) *LinkStatus {
	return typed.NewResource[LinkStatusSpec, LinkStatusExtension](
		resource.NewMetadata(resources.EphemeralNamespace, LinkStatusType, res.Metadata().ID()+"/"+res.Metadata().Type(), resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.LinkStatusSpec{
			LinkId: res.Metadata().ID(),
		}),
	)
}

// LinkStatusType is the type of LinkStatus resource.
//
// tsgen:LinkStatusType
const LinkStatusType = resource.Type("LinkStatuses.omni.sidero.dev")

// LinkStatus resource keeps connected nodes state.
//
// LinkStatus resource ID is a machine UUID.
type LinkStatus = typed.Resource[LinkStatusSpec, LinkStatusExtension]

// LinkStatusSpec wraps specs.LinkStatusSpec.
type LinkStatusSpec = protobuf.ResourceSpec[specs.LinkStatusSpec, *specs.LinkStatusSpec]

// LinkStatusExtension providers auxiliary methods for LinkStatus resource.
type LinkStatusExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (LinkStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             LinkStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.EphemeralNamespace,
		PrintColumns:     nil,
	}
}
