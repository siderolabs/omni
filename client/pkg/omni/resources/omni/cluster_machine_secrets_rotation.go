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

// NewClusterMachineSecretsRotation creates a new ClusterMachineSecretsRotation resource.
func NewClusterMachineSecretsRotation(id resource.ID) *ClusterMachineSecretsRotation {
	return typed.NewResource[ClusterMachineSecretsRotationSpec, ClusterMachineSecretsRotationExtension](
		resource.NewMetadata(resources.DefaultNamespace, ClusterMachineSecretsRotationType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.ClusterMachineSecretsRotationSpec{}),
	)
}

// ClusterMachineSecretsRotationType is the type of ClusterMachineSecretsRotation resource.
//
// tsgen:ClusterMachineSecretsRotationType
const ClusterMachineSecretsRotationType = resource.Type("ClusterMachineSecretsRotations.omni.sidero.dev")

// ClusterMachineSecretsRotation resource describes cluster secrets.
//
// ClusterMachineSecretsRotation resource ID is a cluster ID.
type ClusterMachineSecretsRotation = typed.Resource[ClusterMachineSecretsRotationSpec, ClusterMachineSecretsRotationExtension]

// ClusterMachineSecretsRotationSpec wraps specs.ClusterMachineSecretsRotationSpec.
type ClusterMachineSecretsRotationSpec = protobuf.ResourceSpec[specs.ClusterMachineSecretsRotationSpec, *specs.ClusterMachineSecretsRotationSpec]

// ClusterMachineSecretsRotationExtension providers auxiliary methods for the ClusterMachineSecretsRotation resource.
type ClusterMachineSecretsRotationExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (ClusterMachineSecretsRotationExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ClusterMachineSecretsRotationType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
