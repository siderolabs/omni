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

// NewBackupData creates new resource which holds data for the next etcd backup.
func NewBackupData(id resource.ID) *BackupData {
	return typed.NewResource[BackupDataSpec, BackupDataExtension](
		resource.NewMetadata(resources.DefaultNamespace, BackupDataType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.BackupDataSpec{}),
	)
}

const (
	// BackupDataType is the type of the BackupData resource.
	// tsgen:BackupDataType
	BackupDataType = resource.Type("BackupDatas.omni.sidero.dev")
)

// BackupData describes data needed for the etcd backup.
type BackupData = typed.Resource[BackupDataSpec, BackupDataExtension]

// BackupDataSpec wraps specs.BackupDataSpec.
type BackupDataSpec = protobuf.ResourceSpec[specs.BackupDataSpec, *specs.BackupDataSpec]

// BackupDataExtension provides auxiliary methods for BackupData resource.
type BackupDataExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (BackupDataExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             BackupDataType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
