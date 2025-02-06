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

// NewInfraMachineBMCConfig creates a new InfraMachineBMCConfig resource.
func NewInfraMachineBMCConfig(id string) *InfraMachineBMCConfig {
	return typed.NewResource[InfraMachineBMCConfigSpec, InfraMachineBMCConfigExtension](
		resource.NewMetadata(resources.DefaultNamespace, InfraMachineBMCConfigType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.InfraMachineBMCConfigSpec{}),
	)
}

const (
	// InfraMachineBMCConfigType is the type of InfraMachineBMCConfig resource.
	//
	// tsgen:InfraMachineBMCConfigType
	InfraMachineBMCConfigType = resource.Type("InfraMachineBMCConfigs.omni.sidero.dev")
)

// InfraMachineBMCConfig resource describes the resource.
type InfraMachineBMCConfig = typed.Resource[InfraMachineBMCConfigSpec, InfraMachineBMCConfigExtension]

// InfraMachineBMCConfigSpec wraps specs.InfraMachineBMCConfigSpec.
type InfraMachineBMCConfigSpec = protobuf.ResourceSpec[specs.InfraMachineBMCConfigSpec, *specs.InfraMachineBMCConfigSpec]

// InfraMachineBMCConfigExtension providers auxiliary methods for InfraMachineBMCConfig resource.
type InfraMachineBMCConfigExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (InfraMachineBMCConfigExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             InfraMachineBMCConfigType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "IPMI Address",
				JSONPath: "{.ipmi.address}",
			},
			{
				Name:     "IPMI Username",
				JSONPath: "{.ipmi.username}",
			},
		},
	}
}
