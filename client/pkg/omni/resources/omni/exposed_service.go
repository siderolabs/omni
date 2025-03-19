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

// NewExposedService creates new ExposedService resource.
func NewExposedService(ns string, id resource.ID) *ExposedService {
	return typed.NewResource[ExposedServiceSpec, ExposedServiceExtension](
		resource.NewMetadata(ns, ExposedServiceType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.ExposedServiceSpec{}),
	)
}

const (
	// ExposedServiceType is the type of the ExposedService resource.
	// tsgen:ExposedServiceType
	ExposedServiceType = resource.Type("ExposedServices.omni.sidero.dev")
)

// ExposedService holds the information about an exposed service for workload cluster proxying feature.
type ExposedService = typed.Resource[ExposedServiceSpec, ExposedServiceExtension]

// ExposedServiceSpec wraps specs.ExposedServiceSpec.
type ExposedServiceSpec = protobuf.ResourceSpec[specs.ExposedServiceSpec, *specs.ExposedServiceSpec]

// ExposedServiceExtension provides auxiliary methods for ExposedService resource.
type ExposedServiceExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (ExposedServiceExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ExposedServiceType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Port",
				JSONPath: "{.port}",
			},
			{
				Name:     "Label",
				JSONPath: "{.label}",
			},
			{
				Name:     "URL",
				JSONPath: "{.url}",
			},
			{
				Name:     "Error",
				JSONPath: "{.error}",
			},
		},
	}
}
