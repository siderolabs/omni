// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package omni

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
)

// NewNodeForceDestroyRequest creates a new NodeForceDestroyRequest resource.
func NewNodeForceDestroyRequest(id string) *NodeForceDestroyRequest {
	return typed.NewResource[NodeForceDestroyRequestSpec, NodeForceDestroyRequestExtension](
		resource.NewMetadata(resources.DefaultNamespace, NodeForceDestroyRequestType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.NodeForceDestroyRequestSpec{}),
	)
}

const (
	// NodeForceDestroyRequestType is the type of NodeForceDestroyRequest resource.
	//
	// tsgen:NodeForceDestroyRequestType
	NodeForceDestroyRequestType = resource.Type("NodeForceDestroyRequests.omni.sidero.dev")
)

// NodeForceDestroyRequest resource describes the resource.
type NodeForceDestroyRequest = typed.Resource[NodeForceDestroyRequestSpec, NodeForceDestroyRequestExtension]

// NodeForceDestroyRequestSpec wraps specs.NodeForceDestroyRequestSpec.
type NodeForceDestroyRequestSpec = protobuf.ResourceSpec[specs.NodeForceDestroyRequestSpec, *specs.NodeForceDestroyRequestSpec]

// NodeForceDestroyRequestExtension providers auxiliary methods for NodeForceDestroyRequest resource.
type NodeForceDestroyRequestExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (NodeForceDestroyRequestExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             NodeForceDestroyRequestType,
		DefaultNamespace: resources.DefaultNamespace,
	}
}
