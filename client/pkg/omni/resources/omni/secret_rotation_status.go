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

// NewClusterSecretsRotationStatus creates a new ClusterSecretsRotationStatus resource.
func NewClusterSecretsRotationStatus(id resource.ID) *ClusterSecretsRotationStatus {
	return typed.NewResource[ClusterSecretsRotationStatusSpec, ClusterSecretsRotationStatusExtension](
		resource.NewMetadata(resources.DefaultNamespace, ClusterSecretsRotationStatusType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.ClusterSecretsRotationStatusSpec{}),
	)
}

// ClusterSecretsRotationStatusType is the type of ClusterSecretsRotationStatus resource.
//
// tsgen:ClusterSecretsRotationStatusType
const ClusterSecretsRotationStatusType = resource.Type("ClusterSecretsRotationStatuses.omni.sidero.dev")

// ClusterSecretsRotationStatus resource describes CA rotation request.
type ClusterSecretsRotationStatus = typed.Resource[ClusterSecretsRotationStatusSpec, ClusterSecretsRotationStatusExtension]

// ClusterSecretsRotationStatusSpec wraps specs.ClusterSecretsRotationStatusSpec.
type ClusterSecretsRotationStatusSpec = protobuf.ResourceSpec[specs.ClusterSecretsRotationStatusSpec, *specs.ClusterSecretsRotationStatusSpec]

// ClusterSecretsRotationStatusExtension providers auxiliary methods for ClusterSecretsRotationStatus resource.
type ClusterSecretsRotationStatusExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (ClusterSecretsRotationStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ClusterSecretsRotationStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
