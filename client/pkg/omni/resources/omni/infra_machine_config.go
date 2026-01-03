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

// NewInfraMachineConfig creates a new InfraMachineConfig resource.
func NewInfraMachineConfig(id string) *InfraMachineConfig {
	return typed.NewResource[InfraMachineConfigSpec, InfraMachineConfigExtension](
		resource.NewMetadata(resources.DefaultNamespace, InfraMachineConfigType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.InfraMachineConfigSpec{}),
	)
}

const (
	// InfraMachineConfigType is the type of InfraMachineConfig resource.
	//
	// tsgen:InfraMachineConfigType
	InfraMachineConfigType = resource.Type("InfraMachineConfigs.omni.sidero.dev")
)

// InfraMachineConfig resource describes a machineConfig request.
type InfraMachineConfig = typed.Resource[InfraMachineConfigSpec, InfraMachineConfigExtension]

// InfraMachineConfigSpec wraps specs.InfraMachineConfigSpec.
type InfraMachineConfigSpec = protobuf.ResourceSpec[specs.InfraMachineConfigSpec, *specs.InfraMachineConfigSpec]

// InfraMachineConfigExtension providers auxiliary methods for InfraMachineConfig resource.
type InfraMachineConfigExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (InfraMachineConfigExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             InfraMachineConfigType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Power State",
				JSONPath: "{.powerstate}",
			},
			{
				Name:     "Acceptance",
				JSONPath: "{.acceptancestatus}",
			},
			{
				Name:     "Extra Kernel Args",
				JSONPath: "{.extrakernelargs}",
			},
			{
				Name:     "Requested Reboot ID",
				JSONPath: "{.requestedrebootid}",
			},
			{
				Name:     "Cordoned",
				JSONPath: "{.cordoned}",
			},
		},
	}
}
