// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package auth

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
)

const (
	// ConfigID is the resource ID under which the authentication parameters for auth0 & webauthn will be written to COSI state.
	// tsgen:AuthConfigID
	ConfigID = "auth-config"
)

// NewAuthConfig creates new Config state.
func NewAuthConfig() *Config {
	return typed.NewResource[ConfigSpec, ConfigExtension](
		resource.NewMetadata(resources.DefaultNamespace, AuthConfigType, ConfigID, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.AuthConfigSpec{}),
	)
}

const (
	// AuthConfigType is the type of Config resource.
	//
	// tsgen:AuthConfigType
	AuthConfigType = resource.Type("AuthConfigs.omni.sidero.dev")
)

// Config resource is the Omni authentication configuration.
//
// Config resource ID is a human-readable string without white-space that uniquely identifies the installation media.
type Config = typed.Resource[ConfigSpec, ConfigExtension]

// ConfigSpec wraps specs.AuthConfigSpec.
type ConfigSpec = protobuf.ResourceSpec[specs.AuthConfigSpec, *specs.AuthConfigSpec]

// ConfigExtension providers auxiliary methods for Config resource.
type ConfigExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (ConfigExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             AuthConfigType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}

// Enabled check is config settings has any auth enabled.
func Enabled(res *Config) bool {
	spec := res.TypedSpec().Value

	return spec.Auth0.Enabled || spec.Webauthn.Enabled || spec.Saml.Enabled || spec.Oidc.Enabled
}
