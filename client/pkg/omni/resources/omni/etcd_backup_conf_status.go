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

// NewEtcdBackupStoreStatus creates new etcd backup configuration status info.
func NewEtcdBackupStoreStatus() *EtcdBackupStoreStatus {
	return typed.NewResource[EtcdBackupStoreStatusSpec, EtcdBackupStoreStatusExtension](
		resource.NewMetadata(resources.EphemeralNamespace, EtcdBackupStoreStatusType, EtcdBackupStoreStatusID, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.EtcdBackupStoreStatusSpec{}),
	)
}

const (
	// EtcdBackupStoreStatusID is the ID of the EtcdBackupStoreStatus resource.
	// tsgen:EtcdBackupStoreStatusID
	EtcdBackupStoreStatusID = resource.ID("etcdbackup-store-status")

	// EtcdBackupStoreStatusType is the type of the [EtcdBackupStoreStatus] resource.
	// tsgen:EtcdBackupStoreStatusType
	EtcdBackupStoreStatusType = resource.Type("EtcdBackupStoreStatuses.omni.sidero.dev")
)

// EtcdBackupStoreStatus describes etcd backup status.
type EtcdBackupStoreStatus = typed.Resource[EtcdBackupStoreStatusSpec, EtcdBackupStoreStatusExtension]

// EtcdBackupStoreStatusSpec wraps specs.EtcdBackupStoreStatusSpec.
type EtcdBackupStoreStatusSpec = protobuf.ResourceSpec[specs.EtcdBackupStoreStatusSpec, *specs.EtcdBackupStoreStatusSpec]

// EtcdBackupStoreStatusExtension provides auxiliary methods for EtcdBackupStoreStatus resource.
type EtcdBackupStoreStatusExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (EtcdBackupStoreStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             EtcdBackupStoreStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.EphemeralNamespace,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Configuration Name",
				JSONPath: "{.configurationname}",
			},
			{
				Name:     "Configuration Error",
				JSONPath: "{.configurationerror}",
			},
		},
	}
}
