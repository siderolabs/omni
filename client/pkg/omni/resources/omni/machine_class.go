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

// NewMachineClass creates new MachineClass resource.
func NewMachineClass(ns string, id resource.ID) *MachineClass {
	return typed.NewResource[MachineClassSpec, MachineClassExtension](
		resource.NewMetadata(ns, MachineClassType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.MachineClassSpec{}),
	)
}

const (
	// MachineClassType is the type of the MachineClass resource.
	// tsgen:MachineClassType
	MachineClassType = resource.Type("MachineClasses.omni.sidero.dev")
)

// MachineClass describes machine set resource.
type MachineClass = typed.Resource[MachineClassSpec, MachineClassExtension]

// MachineClassSpec wraps specs.MachineClassSpec.
type MachineClassSpec = protobuf.ResourceSpec[specs.MachineClassSpec, *specs.MachineClassSpec]

// MachineClassExtension provides auxiliary methods for MachineClass resource.
type MachineClassExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (MachineClassExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             MachineClassType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
