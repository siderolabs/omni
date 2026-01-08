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

// NewClusterMachineConfig creates new cluster machine config resource.
func NewClusterMachineConfig(id resource.ID) *ClusterMachineConfig {
	return typed.NewResource[ClusterMachineConfigSpec, ClusterMachineConfigExtension](
		resource.NewMetadata(resources.DefaultNamespace, ClusterMachineConfigType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.ClusterMachineConfigSpec{}),
	)
}

const (
	// ClusterMachineConfigType is the type of the ClusterMachineConfig resource.
	// tsgen:ClusterMachineConfigType
	ClusterMachineConfigType = resource.Type("ClusterMachineConfigs.omni.sidero.dev")
)

// ClusterMachineConfig is the final machine config generated from the template, cluster machine, loadbalancer status
// and siderolink config.
type ClusterMachineConfig = typed.Resource[ClusterMachineConfigSpec, ClusterMachineConfigExtension]

// ClusterMachineConfigSpec wraps specs.ClusterMachineConfigSpec.
type ClusterMachineConfigSpec = protobuf.ResourceSpec[specs.ClusterMachineConfigSpec, *specs.ClusterMachineConfigSpec]

// ClusterMachineConfigExtension provides auxiliary methods for ClusterMachineConfig resource.
type ClusterMachineConfigExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (ClusterMachineConfigExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ClusterMachineConfigType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
