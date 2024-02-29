// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package omni

import (
	"fmt"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
)

// NewEtcdBackup creates new etcd backup info.
func NewEtcdBackup(clusterName string, t time.Time) *EtcdBackup {
	return typed.NewResource[EtcdBackupSpec, EtcdBackupExtension](
		resource.NewMetadata(resources.ExternalNamespace, EtcdBackupType, fmt.Sprintf("%s-%d", clusterName, t.Unix()), resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.EtcdBackupSpec{}),
	)
}

const (
	// EtcdBackupType is the type of the EtcdBackup resource.
	// tsgen:EtcdBackupType
	EtcdBackupType = resource.Type("EtcdBackups.omni.sidero.dev")
)

// EtcdBackup describes etcd backup data.
type EtcdBackup = typed.Resource[EtcdBackupSpec, EtcdBackupExtension]

// EtcdBackupSpec wraps specs.EtcdBackupSpec.
type EtcdBackupSpec = protobuf.ResourceSpec[specs.EtcdBackupSpec, *specs.EtcdBackupSpec]

// EtcdBackupExtension provides auxiliary methods for EtcdBackup resource.
type EtcdBackupExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (EtcdBackupExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             EtcdBackupType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.ExternalNamespace,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Created At",
				JSONPath: "{.createdat}",
			},
			{
				Name:     "Snapshot",
				JSONPath: "{.snapshot}",
			},
		},
	}
}
