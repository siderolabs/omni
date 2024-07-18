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

// NewMachineClassStatus creates new MachineClassStatus resource.
func NewMachineClassStatus(ns string, id resource.ID) *MachineClassStatus {
	return typed.NewResource[MachineClassStatusSpec, MachineClassStatusExtension](
		resource.NewMetadata(ns, MachineClassStatusType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.MachineClassStatusSpec{}),
	)
}

const (
	// MachineClassStatusType is the type of the MachineClassStatus resource.
	// tsgen:MachineClassStatusType
	MachineClassStatusType = resource.Type("MachineClassStatuses.omni.sidero.dev")
)

// MachineClassStatus describes machine set resource.
type MachineClassStatus = typed.Resource[MachineClassStatusSpec, MachineClassStatusExtension]

// MachineClassStatusSpec wraps specs.MachineClassStatusSpec.
type MachineClassStatusSpec = protobuf.ResourceSpec[specs.MachineClassStatusSpec, *specs.MachineClassStatusSpec]

// MachineClassStatusExtension provides auxiliary methods for MachineClassStatus resource.
type MachineClassStatusExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (MachineClassStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             MachineClassStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Required Additional Machines",
				JSONPath: "{.requiredadditionalmachines}",
			},
		},
	}
}
