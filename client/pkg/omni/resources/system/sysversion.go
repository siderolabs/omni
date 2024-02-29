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

// NewSysVersion creates new SysVersion state.
func NewSysVersion(ns, id string) *SysVersion {
	return typed.NewResource[SysVersionSpec, SysVersionExtension](
		resource.NewMetadata(ns, SysVersionType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.SysVersionSpec{}),
	)
}

const (
	// SysVersionType is the type of SysVersion resource.
	//
	// tsgen:SysVersionType
	SysVersionType = resource.Type("SysVersions.system.sidero.dev")
	// SysVersionID is the single resource id.
	//
	// tsgen:SysVersionID
	SysVersionID = resource.ID("current")
)

// SysVersion resource describes current DB SysVersion (migrations state).
type SysVersion = typed.Resource[SysVersionSpec, SysVersionExtension]

// SysVersionSpec wraps specs.SysVersionSpec.
type SysVersionSpec = protobuf.ResourceSpec[specs.SysVersionSpec, *specs.SysVersionSpec]

// SysVersionExtension providers auxiliary methods for SysVersion resource.
type SysVersionExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (SysVersionExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             SysVersionType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.EphemeralNamespace,
	}
}
