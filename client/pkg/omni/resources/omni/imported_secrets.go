// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package omni

import (
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"
	"github.com/siderolabs/talos/pkg/machinery/config/generate/secrets"
	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
)

// NewImportedClusterSecrets creates new ImportedClusterSecrets state.
func NewImportedClusterSecrets(ns string, id resource.ID) *ImportedClusterSecrets {
	return typed.NewResource[ImportedClusterSecretsSpec, ImportedClusterSecretsExtension](
		resource.NewMetadata(ns, ImportedClusterSecretsType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.ImportedClusterSecretsSpec{}),
	)
}

// ImportedClusterSecretsType is the type of ImportedClusterSecrets resource.
//
// tsgen:ImportedClusterSecretsType
const ImportedClusterSecretsType = resource.Type("ImportedClusterSecrets.omni.sidero.dev")

// ImportedClusterSecrets resource describes imported cluster secrets.
type ImportedClusterSecrets = typed.Resource[ImportedClusterSecretsSpec, ImportedClusterSecretsExtension]

// ImportedClusterSecretsSpec wraps specs.ImportedClusterSecretsSpec.
type ImportedClusterSecretsSpec = protobuf.ResourceSpec[specs.ImportedClusterSecretsSpec, *specs.ImportedClusterSecretsSpec]

// ImportedClusterSecretsExtension providers auxiliary methods for ImportedClusterSecrets resource.
type ImportedClusterSecretsExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (ImportedClusterSecretsExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ImportedClusterSecretsType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}

// FromImportedSecretsToSecretsBundle converts the resource into generate.SecretsBundle resource.
func FromImportedSecretsToSecretsBundle(ics *ImportedClusterSecrets) (*secrets.Bundle, error) {
	secretBundle := &secrets.Bundle{}

	err := yaml.Unmarshal([]byte(ics.TypedSpec().Value.Data), secretBundle)
	if err != nil {
		return nil, err
	}

	secretBundle.Clock = secrets.NewFixedClock(time.Now())

	return secretBundle, err
}
