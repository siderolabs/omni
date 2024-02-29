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

// NewClusterMachineIdentity creates new cluster machine identity resource.
func NewClusterMachineIdentity(ns string, id resource.ID) *ClusterMachineIdentity {
	return typed.NewResource[ClusterMachineIdentitySpec, ClusterMachineIdentityExtension](
		resource.NewMetadata(ns, ClusterMachineIdentityType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.ClusterMachineIdentitySpec{}),
	)
}

const (
	// ClusterMachineIdentityType is the type of the ClusterMachineIdentity resource.
	// tsgen:ClusterMachineIdentityType
	ClusterMachineIdentityType = resource.Type("ClusterMachineIdentities.omni.sidero.dev")
)

// ClusterMachineIdentity contains machine identifiers when it's part of a cluster.
type ClusterMachineIdentity = typed.Resource[ClusterMachineIdentitySpec, ClusterMachineIdentityExtension]

// ClusterMachineIdentitySpec wraps specs.ClusterMachineIdentitySpec.
type ClusterMachineIdentitySpec = protobuf.ResourceSpec[specs.ClusterMachineIdentitySpec, *specs.ClusterMachineIdentitySpec]

// ClusterMachineIdentityExtension provides auxiliary methods for ClusterMachineIdentity resource.
type ClusterMachineIdentityExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (ClusterMachineIdentityExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ClusterMachineIdentityType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
