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

// NewEtcdBackupStatus creates new etcd backup status info.
func NewEtcdBackupStatus(id resource.ID) *EtcdBackupStatus {
	return typed.NewResource[EtcdBackupStatusSpec, EtcdBackupStatusExtension](
		resource.NewMetadata(resources.DefaultNamespace, EtcdBackupStatusType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.EtcdBackupStatusSpec{}),
	)
}

const (
	// EtcdBackupStatusType is the type of the EtcdBackupStatus resource.
	// tsgen:EtcdBackupStatusType
	EtcdBackupStatusType = resource.Type("EtcdBackupStatuses.omni.sidero.dev")
)

// EtcdBackupStatus describes etcd backup status.
type EtcdBackupStatus = typed.Resource[EtcdBackupStatusSpec, EtcdBackupStatusExtension]

// EtcdBackupStatusSpec wraps specs.EtcdBackupStatusSpec.
type EtcdBackupStatusSpec = protobuf.ResourceSpec[specs.EtcdBackupStatusSpec, *specs.EtcdBackupStatusSpec]

// EtcdBackupStatusExtension provides auxiliary methods for EtcdBackupStatus resource.
type EtcdBackupStatusExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (EtcdBackupStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             EtcdBackupStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Status",
				JSONPath: "{.status}",
			},
			{
				Name:     "Error",
				JSONPath: "{.error}",
			},
			{
				Name:     "Last Backup",
				JSONPath: "{.lastbackuptime}",
			},
			{
				Name:     "Last Backup Attempt",
				JSONPath: "{.lastbackupattempt}",
			},
		},
	}
}
