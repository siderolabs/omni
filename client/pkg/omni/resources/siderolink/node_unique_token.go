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

// NewNodeUniqueToken creates new NodeUniqueToken state.
func NewNodeUniqueToken(id string) *NodeUniqueToken {
	return typed.NewResource[NodeUniqueTokenSpec, NodeUniqueTokenExtension](
		resource.NewMetadata(resources.DefaultNamespace, NodeUniqueTokenType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.NodeUniqueTokenSpec{}),
	)
}

// NodeUniqueTokenType is the type of NodeUniqueToken resource.
const NodeUniqueTokenType = resource.Type("NodeUniqueTokens.omni.sidero.dev")

// NodeUniqueToken resource keeps the generated node unique token.
type NodeUniqueToken = typed.Resource[NodeUniqueTokenSpec, NodeUniqueTokenExtension]

// NodeUniqueTokenSpec wraps specs.NodeUniqueTokenSpec.
type NodeUniqueTokenSpec = protobuf.ResourceSpec[specs.NodeUniqueTokenSpec, *specs.NodeUniqueTokenSpec]

// NodeUniqueTokenExtension providers auxiliary methods for NodeUniqueToken resource.
type NodeUniqueTokenExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (NodeUniqueTokenExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             NodeUniqueTokenType,
		Aliases:          []resource.Type{},
		DefaultNamespace: Namespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
