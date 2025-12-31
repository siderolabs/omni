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

// NewClusterBootstrapStatus creates new cluster bootstrap status resource.
func NewClusterBootstrapStatus(id resource.ID) *ClusterBootstrapStatus {
	return typed.NewResource[ClusterBootstrapStatusSpec, ClusterBootstrapStatusExtension](
		resource.NewMetadata(resources.DefaultNamespace, ClusterBootstrapStatusType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.ClusterBootstrapStatusSpec{}),
	)
}

const (
	// ClusterBootstrapStatusType is the type of the ClusterMachineConfigStatus resource.
	// tsgen:ClusterBootstrapStatusType
	ClusterBootstrapStatusType = resource.Type("ClusterBootstrapStatuses.omni.sidero.dev")
)

// ClusterBootstrapStatus describes a cluster machine status.
type ClusterBootstrapStatus = typed.Resource[ClusterBootstrapStatusSpec, ClusterBootstrapStatusExtension]

// ClusterBootstrapStatusSpec wraps specs.ClusterBootstrapStatusSpec.
type ClusterBootstrapStatusSpec = protobuf.ResourceSpec[specs.ClusterBootstrapStatusSpec, *specs.ClusterBootstrapStatusSpec]

// ClusterBootstrapStatusExtension provides auxiliary methods for ClusterBootstrapStatus resource.
type ClusterBootstrapStatusExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (ClusterBootstrapStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ClusterBootstrapStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Bootstrapped",
				JSONPath: "{.bootstrapped}",
			},
		},
	}
}
