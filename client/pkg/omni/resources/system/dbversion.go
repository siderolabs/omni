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

// NewDBVersion creates new DBVersion state.
func NewDBVersion(id string) *DBVersion {
	return typed.NewResource[DBVersionSpec, DBVersionExtension](
		resource.NewMetadata(resources.DefaultNamespace, DBVersionType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.DBVersionSpec{}),
	)
}

const (
	// DBVersionType is the type of DBVersion resource.
	DBVersionType = resource.Type("DBVersions.system.sidero.dev")
	// DBVersionID is the single resource id.
	DBVersionID = resource.ID("current")
)

// DBVersion resource describes current DB version (migrations state).
type DBVersion = typed.Resource[DBVersionSpec, DBVersionExtension]

// DBVersionSpec wraps specs.DBVersionSpec.
type DBVersionSpec = protobuf.ResourceSpec[specs.DBVersionSpec, *specs.DBVersionSpec]

// DBVersionExtension providers auxiliary methods for DBVersion resource.
type DBVersionExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (DBVersionExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             DBVersionType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
	}
}
