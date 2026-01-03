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

// NewMachineStatusSnapshot creates new MachineStatusSnapshot state.
func NewMachineStatusSnapshot(id string) *MachineStatusSnapshot {
	return typed.NewResource[MachineStatusSnapshotSpec, MachineStatusSnapshotExtension](
		resource.NewMetadata(resources.DefaultNamespace, MachineStatusSnapshotType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.MachineStatusSnapshotSpec{}),
	)
}

// MachineStatusSnapshotType is the type of MachineStatusSnapshot resource.
//
// tsgen:MachineStatusSnapshotType
const MachineStatusSnapshotType = resource.Type("MachineStatusSnapshots.omni.sidero.dev")

// MachineStatusSnapshot resource contains snapshot of Talos MachineStatus resource.
//
// MachineStatusSnapshot resource ID is a Machine UUID.
type MachineStatusSnapshot = typed.Resource[MachineStatusSnapshotSpec, MachineStatusSnapshotExtension]

// MachineStatusSnapshotSpec wraps specs.MachineStatusSnapshotSpec.
type MachineStatusSnapshotSpec = protobuf.ResourceSpec[specs.MachineStatusSnapshotSpec, *specs.MachineStatusSnapshotSpec]

// MachineStatusSnapshotExtension providers auxiliary methods for MachineStatusSnapshot resource.
type MachineStatusSnapshotExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (MachineStatusSnapshotExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             MachineStatusSnapshotType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
