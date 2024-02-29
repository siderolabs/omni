// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package virtual

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
)

// NewClusterPermissions creates a new ClusterPermissions resource.
func NewClusterPermissions(id resource.ID) *ClusterPermissions {
	return typed.NewResource[ClusterPermissionsSpec, ClusterPermissionsExtension](
		resource.NewMetadata(resources.VirtualNamespace, ClusterPermissionsType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.ClusterPermissionsSpec{}),
	)
}

const (
	// ClusterPermissionsType is the type of ClusterPermissions resource.
	//
	// tsgen:ClusterPermissionsType
	ClusterPermissionsType = resource.Type("ClusterPermissions.omni.sidero.dev")
)

// ClusterPermissions resource describes a user's set of permissions on a cluster.
type ClusterPermissions = typed.Resource[ClusterPermissionsSpec, ClusterPermissionsExtension]

// ClusterPermissionsSpec wraps specs.ClusterPermissionsSpec.
type ClusterPermissionsSpec = protobuf.ResourceSpec[specs.ClusterPermissionsSpec, *specs.ClusterPermissionsSpec]

// ClusterPermissionsExtension providers auxiliary methods for ClusterPermissions resource.
type ClusterPermissionsExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (ClusterPermissionsExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ClusterPermissionsType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.VirtualNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
