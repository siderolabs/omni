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

// NewLoadBalancerConfig creates new LoadBalancer state.
func NewLoadBalancerConfig(ns, id string) *LoadBalancerConfig {
	return typed.NewResource[LoadBalancerConfigSpec, LoadBalancerConfigExtension](
		resource.NewMetadata(ns, LoadBalancerConfigType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.LoadBalancerConfigSpec{}),
	)
}

// LoadBalancerConfigType is a resource type that contains the configuration of a load balancer.
const LoadBalancerConfigType = resource.Type("LoadBalancerConfigs.omni.sidero.dev")

// LoadBalancerConfig is a resource type that contains the configuration of a load balancer.
type LoadBalancerConfig = typed.Resource[LoadBalancerConfigSpec, LoadBalancerConfigExtension]

// LoadBalancerConfigSpec wraps specs.LoadBalancerConfigSpec.
type LoadBalancerConfigSpec = protobuf.ResourceSpec[specs.LoadBalancerConfigSpec, *specs.LoadBalancerConfigSpec]

// LoadBalancerConfigExtension providers auxiliary methods for LoadBalancerConfig resource.
type LoadBalancerConfigExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (LoadBalancerConfigExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             LoadBalancerConfigType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
