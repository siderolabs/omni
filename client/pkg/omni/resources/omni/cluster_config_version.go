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

// NewClusterConfigVersion creates new cluster config version resource.
func NewClusterConfigVersion(id resource.ID) *ClusterConfigVersion {
	return typed.NewResource[ClusterConfigVersionSpec, ClusterConfigVersionExtension](
		resource.NewMetadata(resources.DefaultNamespace, ClusterConfigVersionType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.ClusterConfigVersionSpec{}),
	)
}

const (
	// ClusterConfigVersionType is the type of the ClusterConfigVersion resource.
	// tsgen:ClusterConfigVersionType
	ClusterConfigVersionType = resource.Type("ClusterConfigVersions.omni.sidero.dev")
)

// ClusterConfigVersion contains the version contract for the time when the cluster was created initially.
type ClusterConfigVersion = typed.Resource[ClusterConfigVersionSpec, ClusterConfigVersionExtension]

// ClusterConfigVersionSpec wraps specs.ClusterConfigVersionSpec.
type ClusterConfigVersionSpec = protobuf.ResourceSpec[specs.ClusterConfigVersionSpec, *specs.ClusterConfigVersionSpec]

// ClusterConfigVersionExtension provides auxiliary methods for ClusterConfigVersion resource.
type ClusterConfigVersionExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (ClusterConfigVersionExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ClusterConfigVersionType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns:     nil,
	}
}
