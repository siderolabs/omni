// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package omni

import (
	"encoding/json"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"
	"github.com/siderolabs/talos/pkg/machinery/config/generate/secrets"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
)

// NewClusterSecrets creates new ClusterSecrets state.
func NewClusterSecrets(id resource.ID) *ClusterSecrets {
	return typed.NewResource[ClusterSecretsSpec, ClusterSecretsExtension](
		resource.NewMetadata(resources.DefaultNamespace, ClusterSecretsType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.ClusterSecretsSpec{}),
	)
}

// ClusterSecretsType is the type of ClusterSecrets resource.
//
// tsgen:ClusterSecretsType
const ClusterSecretsType = resource.Type("ClusterSecrets.omni.sidero.dev")

// ClusterSecrets resource describes cluster secrets.
//
// ClusterSecrets resource ID is a cluster ID.
type ClusterSecrets = typed.Resource[ClusterSecretsSpec, ClusterSecretsExtension]

// ClusterSecretsSpec wraps specs.ClusterSecretsSpec.
type ClusterSecretsSpec = protobuf.ResourceSpec[specs.ClusterSecretsSpec, *specs.ClusterSecretsSpec]

// ClusterSecretsExtension providers auxiliary methods for ClusterSecrets resource.
type ClusterSecretsExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (ClusterSecretsExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ClusterSecretsType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}

// ToSecretsBundle decodes the resource into generate.SecretsBundle resource.
func ToSecretsBundle(clusterSecrets *ClusterSecrets) (*secrets.Bundle, error) {
	secretBundle := &secrets.Bundle{}

	err := json.Unmarshal(clusterSecrets.TypedSpec().Value.Data, secretBundle)
	if err != nil {
		return nil, err
	}

	secretBundle.Clock = secrets.NewFixedClock(time.Now())

	return secretBundle, err
}
