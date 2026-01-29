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

// NewSecretRotation creates a new SecretRotation resource.
func NewSecretRotation(id resource.ID) *SecretRotation {
	return typed.NewResource[SecretRotationSpec, SecretRotationExtension](
		resource.NewMetadata(resources.DefaultNamespace, SecretRotationType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.SecretRotationSpec{}),
	)
}

// SecretRotationType is the type of SecretRotation resource.
//
// tsgen:SecretRotationType
const SecretRotationType = resource.Type("SecretRotations.omni.sidero.dev")

// SecretRotation resource holds the secret rotation configuration and state for the cluster.
//
// SecretRotation resource ID is a cluster ID.
type SecretRotation = typed.Resource[SecretRotationSpec, SecretRotationExtension]

// SecretRotationSpec wraps specs.SecretRotationSpec.
type SecretRotationSpec = protobuf.ResourceSpec[specs.SecretRotationSpec, *specs.SecretRotationSpec]

// SecretRotationExtension providers auxiliary methods for the SecretRotation resource.
type SecretRotationExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (SecretRotationExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             SecretRotationType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
