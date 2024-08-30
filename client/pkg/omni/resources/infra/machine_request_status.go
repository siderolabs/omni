// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package infra

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/omni/client/api/omni/specs/infra"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
)

// NewMachineRequestStatus creates a new MachineRequestStatus resource.
func NewMachineRequestStatus(id string) *MachineRequestStatus {
	return typed.NewResource[MachineRequestStatusSpec, MachineRequestStatusExtension](
		resource.NewMetadata(resources.InfraProviderNamespace, MachineRequestStatusType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&infra.MachineRequestStatusSpec{}),
	)
}

const (
	// MachineRequestStatusType is the type of MachineRequestStatus resource.
	//
	// tsgen:MachineRequestStatusType
	MachineRequestStatusType = resource.Type("MachineRequestStatuses.omni.sidero.dev")
)

// MachineRequestStatus resource describes a machine request status.
type MachineRequestStatus = typed.Resource[MachineRequestStatusSpec, MachineRequestStatusExtension]

// MachineRequestStatusSpec wraps specs.MachineRequestStatusSpec.
type MachineRequestStatusSpec = protobuf.ResourceSpec[infra.MachineRequestStatusSpec, *infra.MachineRequestStatusSpec]

// MachineRequestStatusExtension providers auxiliary methods for MachineRequestStatus resource.
type MachineRequestStatusExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (MachineRequestStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             MachineRequestStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.InfraProviderNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
