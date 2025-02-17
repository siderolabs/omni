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

// NewMaintenanceConfigStatus creates a new MaintenanceConfigStatus resource.
func NewMaintenanceConfigStatus(id resource.ID) *MaintenanceConfigStatus {
	return typed.NewResource[MaintenanceConfigStatusSpec, MaintenanceConfigStatusExtension](
		resource.NewMetadata(resources.DefaultNamespace, MaintenanceConfigStatusType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.MaintenanceConfigStatusSpec{}),
	)
}

const (
	// MaintenanceConfigStatusType is the type of the MaintenanceConfigStatus resource.
	// tsgen:MaintenanceConfigStatusType
	MaintenanceConfigStatusType = resource.Type("MaintenanceConfigStatuses.omni.sidero.dev")
)

// MaintenanceConfigStatus describes the spec of the resource.
type MaintenanceConfigStatus = typed.Resource[MaintenanceConfigStatusSpec, MaintenanceConfigStatusExtension]

// MaintenanceConfigStatusSpec wraps specs.MaintenanceConfigStatusSpec.
type MaintenanceConfigStatusSpec = protobuf.ResourceSpec[specs.MaintenanceConfigStatusSpec, *specs.MaintenanceConfigStatusSpec]

// MaintenanceConfigStatusExtension provides auxiliary methods for MaintenanceConfigStatus resource.
type MaintenanceConfigStatusExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (MaintenanceConfigStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             MaintenanceConfigStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Public Key At Last Apply",
				JSONPath: "{.publickeyatlastapply}",
			},
		},
	}
}
