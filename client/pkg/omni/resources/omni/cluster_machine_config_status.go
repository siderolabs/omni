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

// NewClusterMachineConfigStatus creates new cluster machine status resource.
func NewClusterMachineConfigStatus(ns string, id resource.ID) *ClusterMachineConfigStatus {
	return typed.NewResource[ClusterMachineConfigStatusSpec, ClusterMachineConfigStatusExtension](
		resource.NewMetadata(ns, ClusterMachineConfigStatusType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.ClusterMachineConfigStatusSpec{}),
	)
}

const (
	// ClusterMachineConfigStatusType is the type of the ClusterMachineConfigStatus resource.
	// tsgen:ClusterMachineConfigStatusType
	ClusterMachineConfigStatusType = resource.Type("ClusterMachineConfigStatuses.omni.sidero.dev")
)

// ClusterMachineConfigStatus describes a cluster machine status.
type ClusterMachineConfigStatus = typed.Resource[ClusterMachineConfigStatusSpec, ClusterMachineConfigStatusExtension]

// ClusterMachineConfigStatusSpec wraps specs.ClusterMachineConfigStatusSpec.
type ClusterMachineConfigStatusSpec = protobuf.ResourceSpec[specs.ClusterMachineConfigStatusSpec, *specs.ClusterMachineConfigStatusSpec]

// ClusterMachineConfigStatusExtension provides auxiliary methods for ClusterMachineConfigStatus resource.
type ClusterMachineConfigStatusExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (ClusterMachineConfigStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ClusterMachineConfigStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
