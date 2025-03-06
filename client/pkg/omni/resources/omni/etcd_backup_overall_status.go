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

// NewEtcdBackupOverallStatus creates new etcd backup status info.
func NewEtcdBackupOverallStatus() *EtcdBackupOverallStatus {
	return typed.NewResource[EtcdBackupOverallStatusSpec, EtcdBackupOverallStatusExtension](
		resource.NewMetadata(resources.MetricsNamespace, EtcdBackupOverallStatusType, EtcdBackupOverallStatusID, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.EtcdBackupOverallStatusSpec{}),
	)
}

const (
	// EtcdBackupOverallStatusID is the ID of the EtcdBackupOverallStatus resource.
	// tsgen:EtcdBackupOverallStatusID
	EtcdBackupOverallStatusID = resource.ID("etcdbackup-overall-status")

	// EtcdBackupOverallStatusType is the type of the [EtcdBackupOverallStatus] resource.
	// tsgen:EtcdBackupOverallStatusType
	EtcdBackupOverallStatusType = resource.Type("EtcdBackupOverallStatuses.omni.sidero.dev")
)

// EtcdBackupOverallStatus describes etcd backup status.
type EtcdBackupOverallStatus = typed.Resource[EtcdBackupOverallStatusSpec, EtcdBackupOverallStatusExtension]

// EtcdBackupOverallStatusSpec wraps specs.EtcdBackupOverallStatusSpec.
type EtcdBackupOverallStatusSpec = protobuf.ResourceSpec[specs.EtcdBackupOverallStatusSpec, *specs.EtcdBackupOverallStatusSpec]

// EtcdBackupOverallStatusExtension provides auxiliary methods for EtcdBackupOverallStatus resource.
type EtcdBackupOverallStatusExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (EtcdBackupOverallStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             EtcdBackupOverallStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.MetricsNamespace,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Configuration Name",
				JSONPath: "{.configurationname}",
			},
			{
				Name:     "Configuration Error",
				JSONPath: "{.configurationerror}",
			},
			{
				Name:     "Last Backup Status",
				JSONPath: "{.lastbackupstatus.status}",
			},
			{
				Name:     "Last Backup Error",
				JSONPath: "{.lastbackupstatus.error}",
			},
			{
				Name:     "Last Backup Time",
				JSONPath: "{.lastbackupstatus.lastbackuptime}",
			},
			{
				Name:     "Configuration Attempt",
				JSONPath: "{.lastbackupstatus.lastbackupattempt}",
			},
		},
	}
}
