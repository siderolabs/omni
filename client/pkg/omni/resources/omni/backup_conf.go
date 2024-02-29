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

// NewEtcdBackupS3Conf creates new resource which holds for backup s3 configuration.
func NewEtcdBackupS3Conf() *EtcdBackupS3Conf {
	return typed.NewResource[EtcdBackupS3ConfSpec, EtcdBackupS3ConfExtension](
		resource.NewMetadata(resources.DefaultNamespace, EtcdBackupS3ConfType, EtcdBackupS3ConfID, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.EtcdBackupS3ConfSpec{}),
	)
}

const (
	// EtcdBackupS3ConfID is the ID of the EtcdBackupS3Conf resource.
	// tsgen:EtcdBackupS3ConfID
	EtcdBackupS3ConfID = resource.ID("etcd-backup-s3-conf")

	// EtcdBackupS3ConfType is the type of the EtcdBackupS3Conf resource.
	// tsgen:EtcdBackupS3ConfType
	EtcdBackupS3ConfType = resource.Type("EtcdBackupS3Configs.omni.sidero.dev")
)

// EtcdBackupS3Conf describes data needed for the etcd backup.
type EtcdBackupS3Conf = typed.Resource[EtcdBackupS3ConfSpec, EtcdBackupS3ConfExtension]

// EtcdBackupS3ConfSpec wraps specs.EtcdBackupS3ConfSpec.
type EtcdBackupS3ConfSpec = protobuf.ResourceSpec[specs.EtcdBackupS3ConfSpec, *specs.EtcdBackupS3ConfSpec]

// EtcdBackupS3ConfExtension provides auxiliary methods for EtcdBackupS3Conf resource.
type EtcdBackupS3ConfExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (EtcdBackupS3ConfExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             EtcdBackupS3ConfType,
		DefaultNamespace: resources.DefaultNamespace,
		Aliases:          []resource.Type{},
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Bucket",
				JSONPath: "{.bucket}",
			},
			{
				Name:     "Region",
				JSONPath: "{.region}",
			},
			{
				Name:     "Endpoint",
				JSONPath: "{.endpoint}",
			},
			{
				Name:     "Access Key ID",
				JSONPath: "{.accesskeyid}",
			},
			{
				Name:     "Session Token",
				JSONPath: "{.sessiontoken}",
			},
		},
		Sensitivity: meta.Sensitive,
	}
}
