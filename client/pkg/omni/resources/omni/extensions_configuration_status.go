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

// NewExtensionsConfigurationStatus creates new extensions configuration resource.
func NewExtensionsConfigurationStatus(ns string, id resource.ID) *ExtensionsConfigurationStatus {
	return typed.NewResource[ExtensionsConfigurationStatusSpec, ExtensionsConfigurationStatusExtension](
		resource.NewMetadata(ns, ExtensionsConfigurationStatusType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.ExtensionsConfigurationStatusSpec{}),
	)
}

const (
	// ExtensionsConfigurationStatusType is the type of the ExtensionsConfigurationStatus resource.
	// tsgen:ExtensionsConfigurationStatusType
	ExtensionsConfigurationStatusType = resource.Type("ExtensionsConfigurationStatuses.omni.sidero.dev")
)

// ExtensionsConfigurationStatus describes desired machine extensions list for a particular machine, machine set or cluster.
type ExtensionsConfigurationStatus = typed.Resource[ExtensionsConfigurationStatusSpec, ExtensionsConfigurationStatusExtension]

// ExtensionsConfigurationStatusSpec wraps specs.ExtensionsConfigurationStatusSpec.
type ExtensionsConfigurationStatusSpec = protobuf.ResourceSpec[specs.ExtensionsConfigurationStatusSpec, *specs.ExtensionsConfigurationStatusSpec]

// ExtensionsConfigurationStatusExtension provides auxiliary methods for ExtensionsConfigurationStatus resource.
type ExtensionsConfigurationStatusExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (ExtensionsConfigurationStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ExtensionsConfigurationStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Extensions",
				JSONPath: "{.extensions}",
			},
			{
				Name:     "Phase",
				JSONPath: "{.phase}",
			},
		},
	}
}
