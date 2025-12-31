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

// NewClusterMachineEncryptionKey creates new cluster machine encryption key resource.
func NewClusterMachineEncryptionKey(id resource.ID) *ClusterMachineEncryptionKey {
	return typed.NewResource[ClusterMachineEncryptionKeySpec, ClusterMachineEncryptionKeyExtension](
		resource.NewMetadata(resources.DefaultNamespace, ClusterMachineEncryptionKeyType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.ClusterMachineEncryptionKeySpec{}),
	)
}

const (
	// ClusterMachineEncryptionKeyType is the type of the ClusterMachineEncryptionKey resource.
	// tsgen:ClusterMachineEncryptionKeyType
	ClusterMachineEncryptionKeyType = resource.Type("ClusterMachineEncryptionKeys.omni.sidero.dev")
)

// ClusterMachineEncryptionKey is the generated AES256 encryption key.
type ClusterMachineEncryptionKey = typed.Resource[ClusterMachineEncryptionKeySpec, ClusterMachineEncryptionKeyExtension]

// ClusterMachineEncryptionKeySpec wraps specs.ClusterMachineEncryptionKeySpec.
type ClusterMachineEncryptionKeySpec = protobuf.ResourceSpec[specs.ClusterMachineEncryptionKeySpec, *specs.ClusterMachineEncryptionKeySpec]

// ClusterMachineEncryptionKeyExtension provides auxiliary methods for ClusterMachineEncryptionKey resource.
type ClusterMachineEncryptionKeyExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (ClusterMachineEncryptionKeyExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ClusterMachineEncryptionKeyType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
