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

// NewClusterStatus creates new cluster machine status resource.
func NewClusterStatus(ns string, id resource.ID) *ClusterStatus {
	return typed.NewResource[ClusterStatusSpec, ClusterStatusExtension](
		resource.NewMetadata(ns, ClusterStatusType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.ClusterStatusSpec{}),
	)
}

const (
	// ClusterStatusType is the type of the ClusterStatus resource.
	// tsgen:ClusterStatusType
	ClusterStatusType = resource.Type("ClusterStatuses.omni.sidero.dev")
)

// ClusterStatus contains the summary for the cluster health, availability, number of nodes.
type ClusterStatus = typed.Resource[ClusterStatusSpec, ClusterStatusExtension]

// ClusterStatusSpec wraps specs.ClusterStatusSpec.
type ClusterStatusSpec = protobuf.ResourceSpec[specs.ClusterStatusSpec, *specs.ClusterStatusSpec]

// ClusterStatusExtension provides auxiliary methods for ClusterStatus resource.
type ClusterStatusExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (ClusterStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ClusterStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Available",
				JSONPath: "{.available}",
			},
			{
				Name:     "Phase",
				JSONPath: "{.phase}",
			},
			{
				Name:     "Machines",
				JSONPath: "{.machines.total}",
			},
			{
				Name:     "Healthy",
				JSONPath: "{.machines.healthy}",
			},
		},
	}
}

// Make implements [typed.Maker] interface.
func (ClusterStatusExtension) Make(_ *resource.Metadata, spec *ClusterStatusSpec) any {
	return (*clusterStatusAux)(spec)
}

type clusterStatusAux ClusterStatusSpec

func (m *clusterStatusAux) Match(searchFor string) bool {
	val := m.Value

	if searchFor == "ready" {
		return val.Ready
	}

	if searchFor == "!ready" {
		return !val.Ready
	}

	return false
}
