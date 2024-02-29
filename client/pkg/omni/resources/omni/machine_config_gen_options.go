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

// NewMachineConfigGenOptions creates new MachineConfigGenOptions resource.
func NewMachineConfigGenOptions(ns string, id resource.ID) *MachineConfigGenOptions {
	return typed.NewResource[MachineConfigGenOptionsSpec, MachineConfigGenOptionsExtension](
		resource.NewMetadata(ns, MachineConfigGenOptionsType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.MachineConfigGenOptionsSpec{}),
	)
}

const (
	// MachineConfigGenOptionsType is the type of the MachineConfigGenOptions resource.
	// tsgen:MachineConfigGenOptionsType
	MachineConfigGenOptionsType = resource.Type("MachineConfigGenOptions.omni.sidero.dev")
)

// MachineConfigGenOptions describes machine config resource.
type MachineConfigGenOptions = typed.Resource[MachineConfigGenOptionsSpec, MachineConfigGenOptionsExtension]

// MachineConfigGenOptionsSpec wraps specs.MachineConfigGenOptionsSpec.
type MachineConfigGenOptionsSpec = protobuf.ResourceSpec[specs.MachineConfigGenOptionsSpec, *specs.MachineConfigGenOptionsSpec]

// MachineConfigGenOptionsExtension provides auxiliary methods for MachineConfigGenOptions resource.
type MachineConfigGenOptionsExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (MachineConfigGenOptionsExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             MachineConfigGenOptionsType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
