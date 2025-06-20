// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package infra

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
)

// NewMachineRegistration creates a new MachineRegistration resource.
func NewMachineRegistration(id string) *MachineRegistration {
	return typed.NewResource[MachineRegistrationSpec, MachineRegistrationExtension](
		resource.NewMetadata(resources.InfraProviderNamespace, MachineRegistrationType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.MachineRegistrationSpec{}),
	)
}

const (
	// MachineRegistrationType is the type of MachineRegistration resource.
	//
	// tsgen:MachineRegistrationType
	MachineRegistrationType = resource.Type("InfraMachineRegistrations.omni.sidero.dev")
)

// MachineRegistration contains the mininum data extracted from the machine resource required by the infra providers.
type MachineRegistration = typed.Resource[MachineRegistrationSpec, MachineRegistrationExtension]

// MachineRegistrationExtension providers auxiliary methods for MachineRegistration resource.
type MachineRegistrationExtension struct{}

// MachineRegistrationSpec wraps specs.MachineRegistrationSpec.
type MachineRegistrationSpec = protobuf.ResourceSpec[specs.MachineRegistrationSpec, *specs.MachineRegistrationSpec]

// ResourceDefinition implements [typed.Extension] interface.
func (MachineRegistrationExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             MachineRegistrationType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.InfraProviderNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
