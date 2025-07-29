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

// NewNodeUniqueTokenStatus creates new NodeUniqueTokenStatus state.
func NewNodeUniqueTokenStatus(id string) *NodeUniqueTokenStatus {
	return typed.NewResource[NodeUniqueTokenStatusSpec, NodeUniqueTokenStatusExtension](
		resource.NewMetadata(resources.DefaultNamespace, NodeUniqueTokenStatusType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.NodeUniqueTokenStatusSpec{}),
	)
}

// NodeUniqueTokenStatusType is the type of NodeUniqueTokenStatus resource.
const NodeUniqueTokenStatusType = resource.Type("NodeUniqueTokenStatuses.omni.sidero.dev")

// NodeUniqueTokenStatus resource keeps the node unique token status.
type NodeUniqueTokenStatus = typed.Resource[NodeUniqueTokenStatusSpec, NodeUniqueTokenStatusExtension]

// NodeUniqueTokenStatusSpec wraps specs.NodeUniqueTokenStatusSpec.
type NodeUniqueTokenStatusSpec = protobuf.ResourceSpec[specs.NodeUniqueTokenStatusSpec, *specs.NodeUniqueTokenStatusSpec]

// NodeUniqueTokenStatusExtension providers auxiliary methods for NodeUniqueTokenStatus resource.
type NodeUniqueTokenStatusExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (NodeUniqueTokenStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             NodeUniqueTokenStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: Namespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
