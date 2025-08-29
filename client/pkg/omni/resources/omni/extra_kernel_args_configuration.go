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

// NewExtraKernelArgsConfiguration creates new ExtraKernelArgs configuration resource.
func NewExtraKernelArgsConfiguration(ns string, id resource.ID) *ExtraKernelArgsConfiguration {
	return typed.NewResource[ExtraKernelArgsConfigurationSpec, ExtraKernelArgsConfigurationExtension](
		resource.NewMetadata(ns, ExtraKernelArgsConfigurationType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.ExtraKernelArgsConfigurationSpec{}),
	)
}

const (
	// ExtraKernelArgsConfigurationType is the type of the ExtraKernelArgsConfiguration resource.
	// tsgen:ExtraKernelArgsConfigurationType
	ExtraKernelArgsConfigurationType = resource.Type("ExtraKernelArgsConfigurations.omni.sidero.dev")
)

// ExtraKernelArgsConfiguration describes desired machine ExtraKernelArgs list for a particular machine, machine set or cluster.
type ExtraKernelArgsConfiguration = typed.Resource[ExtraKernelArgsConfigurationSpec, ExtraKernelArgsConfigurationExtension]

// ExtraKernelArgsConfigurationSpec wraps specs.ExtraKernelArgsConfigurationSpec.
type ExtraKernelArgsConfigurationSpec = protobuf.ResourceSpec[specs.ExtraKernelArgsConfigurationSpec, *specs.ExtraKernelArgsConfigurationSpec]

// ExtraKernelArgsConfigurationExtension provides auxiliary methods for ExtraKernelArgsConfiguration resource.
type ExtraKernelArgsConfigurationExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (ExtraKernelArgsConfigurationExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ExtraKernelArgsConfigurationType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Extra Kernel Args",
				JSONPath: "{.extrakernelargs}",
			},
		},
	}
}
