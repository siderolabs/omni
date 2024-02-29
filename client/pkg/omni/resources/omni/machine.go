// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package omni

import (
	"strconv"
	"strings"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
)

// NewMachine creates new Machine state.
func NewMachine(ns, id string) *Machine {
	return typed.NewResource[MachineSpec, MachineExtension](
		resource.NewMetadata(ns, MachineType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.MachineSpec{}),
	)
}

// MachineType is the type of Machine resource.
//
// tsgen:MachineType
const MachineType = resource.Type("Machines.omni.sidero.dev")

// Machine resource describes connected Talos server (bare-metal, VM, RPi, etc).
//
// Machine resource ID is a machine UUID.
type Machine = typed.Resource[MachineSpec, MachineExtension]

// MachineSpec wraps specs.MachineSpec.
type MachineSpec = protobuf.ResourceSpec[specs.MachineSpec, *specs.MachineSpec]

// MachineExtension providers auxiliary methods for Machine resource.
type MachineExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (MachineExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             MachineType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Address",
				JSONPath: "{.managementaddress}",
			},
			{
				Name:     "Connected",
				JSONPath: "{.connected}",
			},
			{
				Name:     "Reboots",
				JSONPath: "{.rebootcount}",
			},
		},
	}
}

// Make implements [typed.Maker] interface.
func (MachineExtension) Make(_ *resource.Metadata, spec *MachineSpec) any {
	return (*machineAux)(spec)
}

type machineAux MachineSpec

func (m *machineAux) Match(searchFor string) bool {
	val := m.Value

	return strings.Contains(val.ManagementAddress, searchFor)
}

func (m *machineAux) Field(fieldName string) (string, bool) {
	val := m.Value

	switch fieldName {
	case "management_address":
		return val.ManagementAddress, true
	case "connected":
		return strconv.FormatBool(val.Connected), true
	default:
		return "", false
	}
}
