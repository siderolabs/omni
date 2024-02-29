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

// NewClusterMachineStatus creates a new ClusterMachineStatus state.
func NewClusterMachineStatus(ns, id string) *ClusterMachineStatus {
	return typed.NewResource[ClusterMachineStatusSpec, ClusterMachineStatusExtension](
		resource.NewMetadata(ns, ClusterMachineStatusType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.ClusterMachineStatusSpec{}),
	)
}

// ClusterMachineStatusType is the type of the ClusterMachineStatus resource.
//
// tsgen:ClusterMachineStatusType
const ClusterMachineStatusType = resource.Type("ClusterMachineStatuses.omni.sidero.dev")

// ClusterMachineStatus resource holds information about a machine relevant to its membership in a cluster.
type ClusterMachineStatus = typed.Resource[ClusterMachineStatusSpec, ClusterMachineStatusExtension]

// ClusterMachineStatusSpec wraps specs.ClusterMachineStatusSpec.
type ClusterMachineStatusSpec = protobuf.ResourceSpec[specs.ClusterMachineStatusSpec, *specs.ClusterMachineStatusSpec]

// ClusterMachineStatusExtension providers auxiliary methods for ClusterMachineStatus resource.
type ClusterMachineStatusExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (ClusterMachineStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ClusterMachineStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Ready",
				JSONPath: "{.ready}",
			},
			{
				Name:     "Stage",
				JSONPath: "{.stage}",
			},
			{
				Name:     "apid",
				JSONPath: "{.apidavailable}",
			},
		},
	}
}
