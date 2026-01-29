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

// NewClusterMachineSecrets creates a new ClusterMachineSecrets resource.
func NewClusterMachineSecrets(id resource.ID) *ClusterMachineSecrets {
	return typed.NewResource[ClusterMachineSecretsSpec, ClusterMachineSecretsExtension](
		resource.NewMetadata(resources.DefaultNamespace, ClusterMachineSecretsType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.ClusterMachineSecretsSpec{}),
	)
}

// ClusterMachineSecretsType is the type of ClusterMachineSecrets resource.
//
// tsgen:ClusterMachineSecretsType
const ClusterMachineSecretsType = resource.Type("ClusterMachineSecrets.omni.sidero.dev")

// ClusterMachineSecrets resource describes cluster machine secrets.
//
// ClusterMachineSecrets resource ID is a cluster ID.
type ClusterMachineSecrets = typed.Resource[ClusterMachineSecretsSpec, ClusterMachineSecretsExtension]

// ClusterMachineSecretsSpec wraps specs.ClusterMachineSecretsSpec.
type ClusterMachineSecretsSpec = protobuf.ResourceSpec[specs.ClusterMachineSecretsSpec, *specs.ClusterMachineSecretsSpec]

// ClusterMachineSecretsExtension providers auxiliary methods for the ClusterMachineSecrets resource.
type ClusterMachineSecretsExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (ClusterMachineSecretsExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ClusterMachineSecretsType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
