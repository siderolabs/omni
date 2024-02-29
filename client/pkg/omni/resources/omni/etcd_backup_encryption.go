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

// NewEtcdBackupEncryption creates new etcd backup encryption info.
func NewEtcdBackupEncryption(ns string, id resource.ID) *EtcdBackupEncryption {
	return typed.NewResource[EtcdBackupEncryptionSpec, EtcdBackupEncryptionExtension](
		resource.NewMetadata(ns, EtcdBackupEncryptionType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.EtcdBackupEncryptionSpec{}),
	)
}

const (
	// EtcdBackupEncryptionType is the type of the EtcdBackupEncryption resource.
	// tsgen:EtcdBackupEncryptionType
	EtcdBackupEncryptionType = resource.Type("EtcdBackupEncryptions.omni.sidero.dev")
)

// EtcdBackupEncryption describes etcd backup encryption data.
type EtcdBackupEncryption = typed.Resource[EtcdBackupEncryptionSpec, EtcdBackupEncryptionExtension]

// EtcdBackupEncryptionSpec wraps specs.EtcdBackupEncryptionSpec.
type EtcdBackupEncryptionSpec = protobuf.ResourceSpec[specs.EtcdBackupEncryptionSpec, *specs.EtcdBackupEncryptionSpec]

// EtcdBackupEncryptionExtension provides auxiliary methods for EtcdBackupEncryption resource.
type EtcdBackupEncryptionExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (EtcdBackupEncryptionExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             EtcdBackupEncryptionType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
