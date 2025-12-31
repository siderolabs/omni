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

// NewClusterMachineTalosVersion creates a new ClusterMachineTalosVersion state.
func NewClusterMachineTalosVersion(id string) *ClusterMachineTalosVersion {
	return typed.NewResource[ClusterMachineTalosVersionSpec, ClusterMachineTalosVersionExtension](
		resource.NewMetadata(resources.DefaultNamespace, ClusterMachineTalosVersionType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.ClusterMachineTalosVersionSpec{}),
	)
}

// ClusterMachineTalosVersionType is the type of the ClusterMachineTalosVersion resource.
//
// tsgen:ClusterMachineTalosVersionType
const ClusterMachineTalosVersionType = resource.Type("ClusterMachineTalosVersions.omni.sidero.dev")

// ClusterMachineTalosVersion resource holds information about a machine relevant to its membership in a cluster.
type ClusterMachineTalosVersion = typed.Resource[ClusterMachineTalosVersionSpec, ClusterMachineTalosVersionExtension]

// ClusterMachineTalosVersionSpec wraps specs.ClusterMachineTalosVersionSpec.
type ClusterMachineTalosVersionSpec = protobuf.ResourceSpec[specs.ClusterMachineTalosVersionSpec, *specs.ClusterMachineTalosVersionSpec]

// ClusterMachineTalosVersionExtension providers auxiliary methods for ClusterMachineTalosVersion resource.
type ClusterMachineTalosVersionExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (ClusterMachineTalosVersionExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ClusterMachineTalosVersionType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
