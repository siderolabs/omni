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

// NewClusterMachineConfigPatches creates new cluster machine config patches resource.
func NewClusterMachineConfigPatches(ns string, id resource.ID) *ClusterMachineConfigPatches {
	return typed.NewResource[ClusterMachineConfigPatchesSpec, ClusterMachineConfigPatchesExtension](
		resource.NewMetadata(ns, ClusterMachineConfigPatchesType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.ClusterMachineConfigPatchesSpec{}),
	)
}

// ClusterMachineConfigPatchesType is the type of the ClusterMachineConfigPatches resource.
// tsgen:ClusterMachineConfigPatchesType
const ClusterMachineConfigPatchesType = resource.Type("ClusterMachineConfigPatches.omni.sidero.dev")

// ClusterMachineConfigPatches describes a config patch list related to the cluster machine.
type ClusterMachineConfigPatches = typed.Resource[ClusterMachineConfigPatchesSpec, ClusterMachineConfigPatchesExtension]

// ClusterMachineConfigPatchesSpec wraps specs.ClusterMachineConfigPatchesSpec.
type ClusterMachineConfigPatchesSpec = protobuf.ResourceSpec[specs.ClusterMachineConfigPatchesSpec, *specs.ClusterMachineConfigPatchesSpec]

// ClusterMachineConfigPatchesExtension provides auxiliary methods for ClusterMachineConfigPatches resource.
type ClusterMachineConfigPatchesExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (ClusterMachineConfigPatchesExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ClusterMachineConfigPatchesType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
