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

// NewLoadBalancerStatus creates new LoadBalancerStatus state.
func NewLoadBalancerStatus(id string) *LoadBalancerStatus {
	return typed.NewResource[LoadBalancerStatusSpec, LoadBalancerStatusExtension](
		resource.NewMetadata(resources.DefaultNamespace, LoadBalancerStatusType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.LoadBalancerStatusSpec{}),
	)
}

// LoadBalancerStatusType is a resource type that reports the status of a load balancer.
const LoadBalancerStatusType = resource.Type("LoadBalancerStatuses.omni.sidero.dev")

// LoadBalancerStatus is a resource that reports the status of a load balancer.
type LoadBalancerStatus = typed.Resource[LoadBalancerStatusSpec, LoadBalancerStatusExtension]

// LoadBalancerStatusSpec wraps specs.LoadBalancerStatusSpec.
type LoadBalancerStatusSpec = protobuf.ResourceSpec[specs.LoadBalancerStatusSpec, *specs.LoadBalancerStatusSpec]

// LoadBalancerStatusExtension providers auxiliary methods for LoadBalancerStatus resource.
type LoadBalancerStatusExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (LoadBalancerStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             LoadBalancerStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Healthy",
				JSONPath: "{.healthy}",
			},
		},
	}
}
