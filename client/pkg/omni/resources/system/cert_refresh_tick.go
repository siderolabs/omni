// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package system

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
)

// NewCertRefreshTick creates new CertRefreshTick state.
func NewCertRefreshTick(id string) *CertRefreshTick {
	return typed.NewResource[CertRefreshTickSpec, CertRefreshTickExtension](
		resource.NewMetadata(resources.EphemeralNamespace, CertRefreshTickType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.CertRefreshTickSpec{}),
	)
}

// CertRefreshTickType is the type of CertRefreshTick resource.
const CertRefreshTickType = resource.Type("CertRefreshTicks.system.sidero.dev")

// CertRefreshTick resource is created when it's time to refresh the certificates.
type CertRefreshTick = typed.Resource[CertRefreshTickSpec, CertRefreshTickExtension]

// CertRefreshTickSpec wraps specs.CertRefreshTickSpec.
type CertRefreshTickSpec = protobuf.ResourceSpec[specs.CertRefreshTickSpec, *specs.CertRefreshTickSpec]

// CertRefreshTickExtension providers auxiliary methods for CertRefreshTick resource.
type CertRefreshTickExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (CertRefreshTickExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             CertRefreshTickType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.EphemeralNamespace,
	}
}
