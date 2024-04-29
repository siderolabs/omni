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

// NewClusterKubernetesNodes creates a new cluster kubernetes nodes resource.
func NewClusterKubernetesNodes(ns string, id resource.ID) *ClusterKubernetesNodes {
	return typed.NewResource[ClusterKubernetesNodesSpec, ClusterKubernetesNodesExtension](
		resource.NewMetadata(ns, ClusterKubernetesNodesType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.ClusterKubernetesNodesSpec{}),
	)
}

// ClusterKubernetesNodesType is the type of the ClusterKubernetesNodes resource.
const ClusterKubernetesNodesType = resource.Type("ClusterKubernetesNodes.omni.sidero.dev")

// ClusterKubernetesNodes describes the nodes in a Kubernetes cluster.
type ClusterKubernetesNodes = typed.Resource[ClusterKubernetesNodesSpec, ClusterKubernetesNodesExtension]

// ClusterKubernetesNodesSpec wraps specs.ClusterKubernetesNodesSpec.
type ClusterKubernetesNodesSpec = protobuf.ResourceSpec[specs.ClusterKubernetesNodesSpec, *specs.ClusterKubernetesNodesSpec]

// ClusterKubernetesNodesExtension provides auxiliary methods for ClusterKubernetesNodes resource.
type ClusterKubernetesNodesExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (ClusterKubernetesNodesExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ClusterKubernetesNodesType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Nodes",
				JSONPath: "{.nodes}",
			},
		},
	}
}
