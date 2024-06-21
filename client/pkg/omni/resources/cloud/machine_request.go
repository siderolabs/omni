// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cloud

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	cloudspecs "github.com/siderolabs/omni/client/api/omni/specs/cloud"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
)

// NewMachineRequest creates a new MachineRequest resource.
func NewMachineRequest(id string) *MachineRequest {
	return typed.NewResource[MachineRequestSpec, MachineRequestExtension](
		resource.NewMetadata(resources.CloudProviderNamespace, MachineRequestType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&cloudspecs.MachineRequestSpec{}),
	)
}

const (
	// MachineRequestType is the type of MachineRequest resource.
	//
	// tsgen:MachineRequestType
	MachineRequestType = resource.Type("MachineRequests.omni.sidero.dev")
)

// MachineRequest resource describes a machine request.
type MachineRequest = typed.Resource[MachineRequestSpec, MachineRequestExtension]

// MachineRequestSpec wraps specs.MachineRequestSpec.
type MachineRequestSpec = protobuf.ResourceSpec[cloudspecs.MachineRequestSpec, *cloudspecs.MachineRequestSpec]

// MachineRequestExtension providers auxiliary methods for MachineRequest resource.
type MachineRequestExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (MachineRequestExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             MachineRequestType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.CloudProviderNamespace,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Talos Version",
				JSONPath: "{.talosversion}",
			},
			{
				Name:     "Schematic ID",
				JSONPath: "{.schematicid}",
			},
		},
	}
}
