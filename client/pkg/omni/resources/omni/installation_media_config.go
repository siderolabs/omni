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

// NewInstallationMediaConfig creates new InstallationMediaConfig state.
func NewInstallationMediaConfig(name string) *InstallationMediaConfig {
	return typed.NewResource[InstallationMediaConfigSpec, InstallationMediaConfigExtension](
		resource.NewMetadata(resources.DefaultNamespace, InstallationMediaConfigType, name, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.InstallationMediaConfigSpec{}),
	)
}

const (
	// InstallationMediaConfigType is the type of InstallationMediaConfig resource.
	//
	// tsgen:InstallationMediaConfigType
	InstallationMediaConfigType = resource.Type("InstallationMediaConfigs.omni.sidero.dev")
)

// InstallationMediaConfig resource describes a saved installation media download preset.
type InstallationMediaConfig = typed.Resource[InstallationMediaConfigSpec, InstallationMediaConfigExtension]

// InstallationMediaConfigSpec wraps specs.InstallationMediaConfigSpec.
type InstallationMediaConfigSpec = protobuf.ResourceSpec[specs.InstallationMediaConfigSpec, *specs.InstallationMediaConfigSpec]

// InstallationMediaConfigExtension providers auxiliary methods for InstallationMediaConfig resource.
type InstallationMediaConfigExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (InstallationMediaConfigExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             InstallationMediaConfigType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
