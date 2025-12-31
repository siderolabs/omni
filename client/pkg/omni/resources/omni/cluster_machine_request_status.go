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

// NewClusterMachineRequestStatus creates a new ClusterMachineRequestStatus state.
func NewClusterMachineRequestStatus(id string) *ClusterMachineRequestStatus {
	return typed.NewResource[ClusterMachineRequestStatusSpec, ClusterMachineRequestStatusExtension](
		resource.NewMetadata(resources.DefaultNamespace, ClusterMachineRequestStatusType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.ClusterMachineRequestStatusSpec{}),
	)
}

// ClusterMachineRequestStatusType is the type of the ClusterMachineRequestStatus resource.
//
// tsgen:ClusterMachineRequestStatusType
const ClusterMachineRequestStatusType = resource.Type("ClusterMachineRequestStatuses.omni.sidero.dev")

// ClusterMachineRequestStatus is the aggregated state of machine request status and machine set.
type ClusterMachineRequestStatus = typed.Resource[ClusterMachineRequestStatusSpec, ClusterMachineRequestStatusExtension]

// ClusterMachineRequestStatusSpec wraps specs.ClusterMachineRequestStatusSpec.
type ClusterMachineRequestStatusSpec = protobuf.ResourceSpec[specs.ClusterMachineRequestStatusSpec, *specs.ClusterMachineRequestStatusSpec]

// ClusterMachineRequestStatusExtension providers auxiliary methods for ClusterMachineRequestStatus resource.
type ClusterMachineRequestStatusExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (ClusterMachineRequestStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ClusterMachineRequestStatusType,
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
