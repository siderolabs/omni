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

const (
	// FeaturesConfigID is the resource ID under which the features configuration will be written to COSI state.
	// tsgen:FeaturesConfigID
	FeaturesConfigID = "features-config"
)

// NewFeaturesConfig creates new FeaturesConfig state.
func NewFeaturesConfig(id resource.ID) *FeaturesConfig {
	return typed.NewResource[FeaturesConfigSpec, FeaturesConfigExtension](
		resource.NewMetadata(resources.DefaultNamespace, FeaturesConfigType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.FeaturesConfigSpec{}),
	)
}

const (
	// FeaturesConfigType is the type of FeaturesConfig resource.
	//
	// tsgen:FeaturesConfigType
	FeaturesConfigType = resource.Type("FeaturesConfigs.omni.sidero.dev")
)

// FeaturesConfig resource describes the features that are enabled in Omni and their configuration.
type FeaturesConfig = typed.Resource[FeaturesConfigSpec, FeaturesConfigExtension]

// FeaturesConfigSpec wraps specs.FeaturesConfigSpec.
type FeaturesConfigSpec = protobuf.ResourceSpec[specs.FeaturesConfigSpec, *specs.FeaturesConfigSpec]

// FeaturesConfigExtension providers auxiliary methods for FeaturesConfig resource.
type FeaturesConfigExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (FeaturesConfigExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             FeaturesConfigType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Workload Proxying",
				JSONPath: "{.enableworkloadproxying}",
			},
		},
	}
}
