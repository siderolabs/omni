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

// NewClusterUUID creates new cluster UUID resource.
func NewClusterUUID(id resource.ID) *ClusterUUID {
	return typed.NewResource[ClusterUUIDSpec, ClusterUUIDExtension](
		resource.NewMetadata(resources.DefaultNamespace, ClusterUUIDType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.ClusterUUID{}),
	)
}

const (
	// ClusterUUIDType is the type of the Cluster UUID resource.
	// tsgen:ClusterUUIDType
	ClusterUUIDType = resource.Type("ClusterUUIDs.omni.sidero.dev")
)

// ClusterUUID describes cluster UUID attached to cluster.
type ClusterUUID = typed.Resource[ClusterUUIDSpec, ClusterUUIDExtension]

// ClusterUUIDSpec wraps specs.ClusterUUID.
type ClusterUUIDSpec = protobuf.ResourceSpec[specs.ClusterUUID, *specs.ClusterUUID]

// ClusterUUIDExtension provides auxiliary methods for ClusterUUID resource.
type ClusterUUIDExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (ClusterUUIDExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ClusterUUIDType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "UUID",
				JSONPath: "{.uuid}",
			},
		},
	}
}
