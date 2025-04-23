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

// NewDiscoveryAffiliateDeleteTask creates a new DiscoveryAffiliateDeleteTask resource.
func NewDiscoveryAffiliateDeleteTask(id resource.ID) *DiscoveryAffiliateDeleteTask {
	return typed.NewResource[DiscoveryAffiliateDeleteTaskSpec, DiscoveryAffiliateDeleteTaskExtension](
		resource.NewMetadata(resources.DefaultNamespace, DiscoveryAffiliateDeleteTaskType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.DiscoveryAffiliateDeleteTaskSpec{}),
	)
}

const (
	// DiscoveryAffiliateDeleteTaskType is the type of the DiscoveryAffiliateDeleteTask resource.
	// tsgen:DiscoveryAffiliateDeleteTaskType
	DiscoveryAffiliateDeleteTaskType = resource.Type("DiscoveryAffiliateDeleteTasks.omni.sidero.dev")
)

// DiscoveryAffiliateDeleteTask describes the spec of the resource.
type DiscoveryAffiliateDeleteTask = typed.Resource[DiscoveryAffiliateDeleteTaskSpec, DiscoveryAffiliateDeleteTaskExtension]

// DiscoveryAffiliateDeleteTaskSpec wraps specs.DiscoveryAffiliateDeleteTaskSpec.
type DiscoveryAffiliateDeleteTaskSpec = protobuf.ResourceSpec[specs.DiscoveryAffiliateDeleteTaskSpec, *specs.DiscoveryAffiliateDeleteTaskSpec]

// DiscoveryAffiliateDeleteTaskExtension provides auxiliary methods for DiscoveryAffiliateDeleteTask resource.
type DiscoveryAffiliateDeleteTaskExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (DiscoveryAffiliateDeleteTaskExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             DiscoveryAffiliateDeleteTaskType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Cluster ID",
				JSONPath: "{.clusterid}",
			},
			{
				Name:     "Discovery Service Endpoint",
				JSONPath: "{.discoveryserviceendpoint}",
			},
		},
	}
}
