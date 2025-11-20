// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package virtual

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
)

// NewSBCConfig creates a new SBCConfig resource.
func NewSBCConfig(id string) *SBCConfig {
	return typed.NewResource[SBCConfigSpec, SBCConfigExtension](
		resource.NewMetadata(resources.VirtualNamespace, SBCConfigType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.SBCConfigSpec{}),
	)
}

const (
	// SBCConfigType is the type of SBCConfig resource.
	//
	// tsgen:SBCConfigType
	SBCConfigType = resource.Type("SBCConfigs.omni.sidero.dev")
)

// SBCConfig resource describes an single board computer configuration for the installation media wizard.
type SBCConfig = typed.Resource[SBCConfigSpec, SBCConfigExtension]

// SBCConfigSpec wraps specs.SBCConfigSpec.
type SBCConfigSpec = protobuf.ResourceSpec[specs.SBCConfigSpec, *specs.SBCConfigSpec]

// SBCConfigExtension providers auxiliary methods for SBCConfig resource.
type SBCConfigExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (SBCConfigExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             SBCConfigType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.VirtualNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
