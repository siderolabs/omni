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

// NewEtcdManualBackup creates new etcd manual backup resource.
func NewEtcdManualBackup(id resource.ID) *EtcdManualBackup {
	return typed.NewResource[EtcdManualBackupSpec, EtcdManualBackupExtension](
		resource.NewMetadata(resources.EphemeralNamespace, EtcdManualBackupType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.EtcdManualBackupSpec{}),
	)
}

const (
	// EtcdManualBackupType is the type of the EtcdManualBackup resource.
	// tsgen:EtcdManualBackupType
	EtcdManualBackupType = resource.Type("EtcdManualBackups.omni.sidero.dev")
)

// EtcdManualBackup describes requested etcd manual backup.
type EtcdManualBackup = typed.Resource[EtcdManualBackupSpec, EtcdManualBackupExtension]

// EtcdManualBackupSpec wraps specs.EtcdManualBackupSpec.
type EtcdManualBackupSpec = protobuf.ResourceSpec[specs.EtcdManualBackupSpec, *specs.EtcdManualBackupSpec]

// EtcdManualBackupExtension provides auxiliary methods for EtcdManualBackup resource.
type EtcdManualBackupExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (EtcdManualBackupExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             EtcdManualBackupType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.EphemeralNamespace,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Backup At",
				JSONPath: "{.backupat}",
			},
		},
	}
}
