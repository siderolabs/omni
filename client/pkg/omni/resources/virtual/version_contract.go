// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package virtual

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
)

// NewVersionContract creates a new VersionContract resource.
func NewVersionContract(id string) *VersionContract {
	return typed.NewResource[VersionContractSpec, VersionContractExtension](
		resource.NewMetadata(resources.VirtualNamespace, VersionContractType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.VersionContractSpec{}),
	)
}

const (
	// VersionContractType is the type of VersionContract resource.
	//
	// tsgen:VersionContractType
	VersionContractType = resource.Type("VersionContract.omni.sidero.dev")
)

// VersionContract resource describes the current Stripe subscription plan.
type VersionContract = typed.Resource[VersionContractSpec, VersionContractExtension]

// VersionContractSpec wraps specs.VersionContractSpec.
type VersionContractSpec = protobuf.ResourceSpec[specs.VersionContractSpec, *specs.VersionContractSpec]

// VersionContractExtension provides auxiliary methods for VersionContract resource.
type VersionContractExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (VersionContractExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             VersionContractType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.VirtualNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
