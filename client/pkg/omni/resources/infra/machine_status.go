// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package infra

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
)

// NewMachineStatus creates a new MachineStatus resource.
func NewMachineStatus(id string) *MachineStatus {
	return typed.NewResource[MachineStatusSpec, MachineStatusExtension](
		resource.NewMetadata(resources.InfraProviderNamespace, InfraMachineStatusType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.InfraMachineStatusSpec{}),
	)
}

const (
	// InfraMachineStatusType is the type of MachineStatus resource.
	//
	// tsgen:InfraMachineStatusType
	InfraMachineStatusType = resource.Type("InfraMachineStatuses.omni.sidero.dev")
)

// MachineStatus resource describes an infra machine status.
type MachineStatus = typed.Resource[MachineStatusSpec, MachineStatusExtension]

// MachineStatusSpec wraps specs.MachineStatusSpec.
type MachineStatusSpec = protobuf.ResourceSpec[specs.InfraMachineStatusSpec, *specs.InfraMachineStatusSpec]

// MachineStatusExtension providers auxiliary methods for MachineStatus resource.
type MachineStatusExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (MachineStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             InfraMachineStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.InfraProviderNamespace,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Power State",
				JSONPath: "{.powerstate}",
			},
			{
				Name:     "Ready to Use",
				JSONPath: "{.readytouse}",
			},
			{
				Name:     "Last Reboot ID",
				JSONPath: "{.lastrebootid}",
			},
			{
				Name:     "Last Reboot At",
				JSONPath: "{.lastreboottimestamp}",
			},
			{
				Name:     "Installed",
				JSONPath: "{.installed}",
			},
		},
	}
}
