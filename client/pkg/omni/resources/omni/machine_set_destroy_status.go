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

// NewMachineSetDestroyStatus creates new cluster destroy status.
func NewMachineSetDestroyStatus(id resource.ID) *MachineSetDestroyStatus {
	return typed.NewResource[MachineSetDestroyStatusSpec, MachineSetDestroyStatusExtension](
		resource.NewMetadata(resources.EphemeralNamespace, MachineSetDestroyStatusType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.DestroyStatusSpec{}),
	)
}

const (
	// MachineSetDestroyStatusType is the type of the MachineSetDestroyStatus resource.
	// tsgen:MachineSetDestroyStatusType
	MachineSetDestroyStatusType = resource.Type("MachineSetDestroyStatuses.omni.sidero.dev")
)

// MachineSetDestroyStatus contains the state of machine set destroy for machine tsse in TearingDown phase.
type MachineSetDestroyStatus = typed.Resource[MachineSetDestroyStatusSpec, MachineSetDestroyStatusExtension]

// MachineSetDestroyStatusSpec wraps specs.MachineSetDestroyStatusSpec.
type MachineSetDestroyStatusSpec = protobuf.ResourceSpec[specs.DestroyStatusSpec, *specs.DestroyStatusSpec]

// MachineSetDestroyStatusExtension provides auxiliary methods for MachineSetDestroyStatus resource.
type MachineSetDestroyStatusExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (MachineSetDestroyStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             MachineSetDestroyStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.EphemeralNamespace,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Phase",
				JSONPath: "{.phase}",
			},
		},
	}
}
