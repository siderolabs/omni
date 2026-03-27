// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package auth

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
)

const (
	// EulaAcceptanceID is the singleton resource ID for the EULA acceptance state.
	// tsgen:EulaAcceptanceID
	EulaAcceptanceID = "eula"

	// EulaAcceptanceType is the type of EulaAcceptance resource.
	//
	// tsgen:EulaAcceptanceType
	EulaAcceptanceType = resource.Type("EulaAcceptances.omni.sidero.dev")
)

// NewEulaAcceptance creates a new EulaAcceptance resource.
func NewEulaAcceptance() *EulaAcceptance {
	return typed.NewResource[EulaAcceptanceSpec, EulaAcceptanceExtension](
		resource.NewMetadata(resources.DefaultNamespace, EulaAcceptanceType, EulaAcceptanceID, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.EulaAcceptanceSpec{}),
	)
}

// EulaAcceptance resource records the instance-wide EULA acceptance.
type EulaAcceptance = typed.Resource[EulaAcceptanceSpec, EulaAcceptanceExtension]

// EulaAcceptanceSpec wraps specs.EulaAcceptanceSpec.
type EulaAcceptanceSpec = protobuf.ResourceSpec[specs.EulaAcceptanceSpec, *specs.EulaAcceptanceSpec]

// EulaAcceptanceExtension provides auxiliary methods for EulaAcceptance resource.
type EulaAcceptanceExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (EulaAcceptanceExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             EulaAcceptanceType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns: []meta.PrintColumn{
			{Name: "Name", JSONPath: "{.acceptedbyname}"},
			{Name: "Email", JSONPath: "{.acceptedbyemail}"},
		},
	}
}
