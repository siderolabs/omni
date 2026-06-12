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

// NewMachineConfigExtractionStatus creates a new MachineConfigExtractionStatus resource.
func NewMachineConfigExtractionStatus(id resource.ID) *MachineConfigExtractionStatus {
	return typed.NewResource[MachineConfigExtractionStatusSpec, MachineConfigExtractionStatusExtension](
		resource.NewMetadata(resources.DefaultNamespace, MachineConfigExtractionStatusType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.MachineConfigExtractionStatusSpec{}),
	)
}

const (
	// MachineConfigExtractionStatusType is the type of the MachineConfigExtractionStatus resource.
	// tsgen:MachineConfigExtractionStatusType
	MachineConfigExtractionStatusType = resource.Type("MachineConfigExtractionStatuses.omni.sidero.dev")
)

// MachineConfigExtractionStatus tracks whether the config a machine arrived with has been extracted into a config patch.
type MachineConfigExtractionStatus = typed.Resource[MachineConfigExtractionStatusSpec, MachineConfigExtractionStatusExtension]

// MachineConfigExtractionStatusSpec wraps specs.MachineConfigExtractionStatusSpec.
type MachineConfigExtractionStatusSpec = protobuf.ResourceSpec[specs.MachineConfigExtractionStatusSpec, *specs.MachineConfigExtractionStatusSpec]

// MachineConfigExtractionStatusExtension provides auxiliary methods for MachineConfigExtractionStatus resource.
type MachineConfigExtractionStatusExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (MachineConfigExtractionStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             MachineConfigExtractionStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Initialized",
				JSONPath: "{.initialized}",
			},
			{
				Name:     "Error",
				JSONPath: "{.error}",
			},
		},
	}
}
