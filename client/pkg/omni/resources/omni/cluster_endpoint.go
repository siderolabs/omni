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

// NewClusterEndpoint creates new cluster machine status resource.
func NewClusterEndpoint(ns string, id resource.ID) *ClusterEndpoint {
	return typed.NewResource[ClusterEndpointSpec, ClusterEndpointExtension](
		resource.NewMetadata(ns, ClusterEndpointType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.ClusterEndpointSpec{}),
	)
}

const (
	// ClusterEndpointType is the type of the ClusterEndpoint resource.
	// tsgen:ClusterEndpointType
	ClusterEndpointType = resource.Type("ClusterEndpoints.omni.sidero.dev")
)

// ClusterEndpoint contains the summary for the cluster health, availability, number of nodes.
type ClusterEndpoint = typed.Resource[ClusterEndpointSpec, ClusterEndpointExtension]

// ClusterEndpointSpec wraps specs.ClusterEndpointSpec.
type ClusterEndpointSpec = protobuf.ResourceSpec[specs.ClusterEndpointSpec, *specs.ClusterEndpointSpec]

// ClusterEndpointExtension provides auxiliary methods for ClusterEndpoint resource.
type ClusterEndpointExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (ClusterEndpointExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ClusterEndpointType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Endpoints",
				JSONPath: "{.managementaddresses}",
			},
		},
	}
}
