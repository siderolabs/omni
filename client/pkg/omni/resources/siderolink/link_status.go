// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package siderolink

import (
	"strings"

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
		resource.NewMetadata(resources.EphemeralNamespace, LinkStatusType, LinkStatusID(res.Metadata().ID(), res.Metadata().Type()), resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.LinkStatusSpec{
			LinkId: res.Metadata().ID(),
		}),
	)
}

// LinkStatusID returns the ID of the LinkStatus of a resource, e.g., of a Link or a PendingMachine.
//
// It contains the type of the resource, as a Link and a PendingMachine can have the same ID.
func LinkStatusID(id resource.ID, typ resource.Type) resource.ID {
	return id + "/" + typ
}

// ParseLinkStatusID returns the ID and the type of the resource a LinkStatus with the given ID is for.
//
// The type is what follows the last slash: a type never contains a slash, but the ID of the resource can,
// e.g., the ID of a PendingMachine is a public key in base64.
func ParseLinkStatusID(linkStatusID resource.ID) (id resource.ID, typ resource.Type, ok bool) {
	return strings.CutLast(linkStatusID, "/")
}

// LinkStatusType is the type of LinkStatus resource.
//
// tsgen:LinkStatusType
const LinkStatusType = resource.Type("LinkStatuses.omni.sidero.dev")

// LinkStatus resource keeps connected nodes state.
//
// LinkStatus resource ID is built by LinkStatusID from the ID and the type of the resource it is for.
type LinkStatus = typed.Resource[LinkStatusSpec, LinkStatusExtension]

// LinkStatusSpec wraps specs.LinkStatusSpec.
type LinkStatusSpec = protobuf.ResourceSpec[specs.LinkStatusSpec, *specs.LinkStatusSpec]

// LinkStatusExtension provides auxiliary methods for LinkStatus resource.
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
