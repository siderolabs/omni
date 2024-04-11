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

// NewClusterIdentity creates a new cluster identity resource.
func NewClusterIdentity(ns string, id resource.ID) *ClusterIdentity {
	return typed.NewResource[ClusterIdentitySpec, ClusterIdentityExtension](
		resource.NewMetadata(ns, ClusterIdentityType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.ClusterIdentitySpec{}),
	)
}

// ClusterIdentityType is the type of the ClusterIdentity resource.
const ClusterIdentityType = resource.Type("ClusterIdentities.omni.sidero.dev")

// ClusterIdentity describes the identity of a cluster.
type ClusterIdentity = typed.Resource[ClusterIdentitySpec, ClusterIdentityExtension]

// ClusterIdentitySpec wraps specs.ClusterIdentitySpec.
type ClusterIdentitySpec = protobuf.ResourceSpec[specs.ClusterIdentitySpec, *specs.ClusterIdentitySpec]

// ClusterIdentityExtension provides auxiliary methods for ClusterIdentity resource.
type ClusterIdentityExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (ClusterIdentityExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ClusterIdentityType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Cluster ID",
				JSONPath: "{.clusterid}",
			},
			{
				Name:     "Node IDs",
				JSONPath: "{.nodeids}",
			},
		},
	}
}
