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

// NewMachineUpgradeStatus creates a new MachineUpgradeStatus resource.
func NewMachineUpgradeStatus(id resource.ID) *MachineUpgradeStatus {
	return typed.NewResource[MachineUpgradeStatusSpec, MachineUpgradeStatusExtension](
		resource.NewMetadata(resources.DefaultNamespace, MachineUpgradeStatusType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.MachineUpgradeStatusSpec{}),
	)
}

const (
	// MachineUpgradeStatusType is the type of the MachineUpgradeStatus resource.
	// tsgen:MachineUpgradeStatusType
	MachineUpgradeStatusType = resource.Type("MachineUpgradeStatuses.omni.sidero.dev")
)

// MachineUpgradeStatus describes the spec of the resource.
type MachineUpgradeStatus = typed.Resource[MachineUpgradeStatusSpec, MachineUpgradeStatusExtension]

// MachineUpgradeStatusSpec wraps specs.MachineUpgradeStatusSpec.
type MachineUpgradeStatusSpec = protobuf.ResourceSpec[specs.MachineUpgradeStatusSpec, *specs.MachineUpgradeStatusSpec]

// MachineUpgradeStatusExtension provides auxiliary methods for MachineUpgradeStatus resource.
type MachineUpgradeStatusExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (MachineUpgradeStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             MachineUpgradeStatusType,
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Schematic ID",
				JSONPath: "{.schematicid}",
			},
			{
				Name:     "Talos Version",
				JSONPath: "{.talosversion}",
			},
			{
				Name:     "Phase",
				JSONPath: "{.phase}",
			},
			{
				Name:     "Status",
				JSONPath: "{.status}",
			},
			{
				Name:     "Error",
				JSONPath: "{.error}",
			},
		},
	}
}
