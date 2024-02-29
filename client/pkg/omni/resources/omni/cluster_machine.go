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

// NewClusterMachine creates new cluster machine resource.
func NewClusterMachine(ns string, id resource.ID) *ClusterMachine {
	return typed.NewResource[ClusterMachineSpec, ClusterMachineExtension](
		resource.NewMetadata(ns, ClusterMachineType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.ClusterMachineSpec{}),
	)
}

// ClusterMachineType is the type of the ClusterMachine resource.
// tsgen:ClusterMachineType
const ClusterMachineType = resource.Type("ClusterMachines.omni.sidero.dev")

// ClusterMachine describes a machine belonging to a cluster.
type ClusterMachine = typed.Resource[ClusterMachineSpec, ClusterMachineExtension]

// ClusterMachineSpec wraps specs.ClusterMachineSpec.
type ClusterMachineSpec = protobuf.ResourceSpec[specs.ClusterMachineSpec, *specs.ClusterMachineSpec]

// ClusterMachineExtension provides auxiliary methods for ClusterMachine resource.
type ClusterMachineExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (ClusterMachineExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ClusterMachineType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
