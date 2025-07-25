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

// NewClusterMachineExtendedConfig creates new cluster machine operation resource.
func NewClusterMachineExtendedConfig(ns string, id resource.ID) *ClusterMachineExtendedConfig {
	return typed.NewResource[ClusterMachineExtendedConfigSpec, ClusterMachineExtendedConfigExtension](
		resource.NewMetadata(ns, ClusterMachineExtendedConfigType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.ClusterMachineExtendedConfigSpec{}),
	)
}

const (
	// ClusterMachineExtendedConfigType is the type of the ClusterMachineExtendedConfig resource.
	// tsgen:ClusterMachineExtendedConfigType
	ClusterMachineExtendedConfigType = resource.Type("ClusterMachineExtendedConfigs.omni.sidero.dev")
)

// ClusterMachineExtendedConfig is the final machine operation generated from the template, cluster machine, loadbalancer status, siderolink Operation
// and install image.
type ClusterMachineExtendedConfig = typed.Resource[ClusterMachineExtendedConfigSpec, ClusterMachineExtendedConfigExtension]

// ClusterMachineExtendedConfigSpec wraps specs.ClusterMachineConfigSpec.
type ClusterMachineExtendedConfigSpec = protobuf.ResourceSpec[specs.ClusterMachineExtendedConfigSpec, *specs.ClusterMachineExtendedConfigSpec]

// ClusterMachineExtendedConfigExtension provides auxiliary methods for ClusterMachineExtendedConfig resource.
type ClusterMachineExtendedConfigExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (ClusterMachineExtendedConfigExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ClusterMachineExtendedConfigType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
