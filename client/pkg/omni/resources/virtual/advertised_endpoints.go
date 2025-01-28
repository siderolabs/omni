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

// AdvertisedEndpointsID is the default and the only allowed ID for AdvertisedEndpoints resource.
//
// tsgen:AdvertisedEndpointsID
const AdvertisedEndpointsID = "current"

// NewAdvertisedEndpoints creates a new AdvertisedEndpoints resource.
func NewAdvertisedEndpoints() *AdvertisedEndpoints {
	return typed.NewResource[AdvertisedEndpointsSpec, AdvertisedEndpointsExtension](
		resource.NewMetadata(resources.VirtualNamespace, AdvertisedEndpointsType, AdvertisedEndpointsID, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.AdvertisedEndpointsSpec{}),
	)
}

const (
	// AdvertisedEndpointsType is the type of AdvertisedEndpoints resource.
	//
	// tsgen:AdvertisedEndpointsType
	AdvertisedEndpointsType = resource.Type("AdvertisedEndpoints.omni.sidero.dev")
)

// AdvertisedEndpoints resource describes the configuration of the advertised endpoints of the Omni instance.
type AdvertisedEndpoints = typed.Resource[AdvertisedEndpointsSpec, AdvertisedEndpointsExtension]

// AdvertisedEndpointsSpec wraps specs.AdvertisedEndpointsSpec.
type AdvertisedEndpointsSpec = protobuf.ResourceSpec[specs.AdvertisedEndpointsSpec, *specs.AdvertisedEndpointsSpec]

// AdvertisedEndpointsExtension providers auxiliary methods for AdvertisedEndpoints resource.
type AdvertisedEndpointsExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (AdvertisedEndpointsExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             AdvertisedEndpointsType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.VirtualNamespace,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "gRPC API URL",
				JSONPath: "{.grpcapiurl}",
			},
		},
	}
}
