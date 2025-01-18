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

// NewMachine creates a new Machine resource.
func NewMachine(id string) *Machine {
	return typed.NewResource[MachineSpec, MachineExtension](
		resource.NewMetadata(resources.InfraProviderNamespace, InfraMachineType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.InfraMachineSpec{}),
	)
}

const (
	// InfraMachineType is the type of Machine resource.
	//
	// tsgen:InfraMachineType
	InfraMachineType = resource.Type("InfraMachines.omni.sidero.dev")
)

// Machine resource describes an infra machine.
type Machine = typed.Resource[MachineSpec, MachineExtension]

// MachineSpec wraps specs.MachineSpec.
type MachineSpec = protobuf.ResourceSpec[specs.InfraMachineSpec, *specs.InfraMachineSpec]

// MachineExtension providers auxiliary methods for Machine resource.
type MachineExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (MachineExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             InfraMachineType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.InfraProviderNamespace,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Preferred Power State",
				JSONPath: "{.preferredpowerstate}",
			},
			{
				Name:     "Acceptance",
				JSONPath: "{.acceptancestatus}",
			},
			{
				Name:     "Cluster Talos Version",
				JSONPath: "{.clustertalosversion}",
			},
			{
				Name:     "Extensions",
				JSONPath: "{.extensions}",
			},
			{
				Name:     "Wipe ID",
				JSONPath: "{.wipeid}",
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
			{
				Name:     "Install Event ID",
				JSONPath: "{.installeventid}",
			},
		},
	}
}
