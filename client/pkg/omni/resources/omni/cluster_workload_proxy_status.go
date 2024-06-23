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

// NewClusterWorkloadProxyStatus creates new ClusterWorkloadProxyStatus resource.
func NewClusterWorkloadProxyStatus(ns string, id resource.ID) *ClusterWorkloadProxyStatus {
	return typed.NewResource[ClusterWorkloadProxyStatusSpec, ClusterWorkloadProxyStatusExtension](
		resource.NewMetadata(ns, ClusterWorkloadProxyStatusType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.ClusterWorkloadProxyStatusSpec{}),
	)
}

const (
	// ClusterWorkloadProxyStatusType is the type of the ClusterWorkloadProxyStatus resource.
	// tsgen:ClusterWorkloadProxyStatusType
	ClusterWorkloadProxyStatusType = resource.Type("ClusterWorkloadProxyStatuses.omni.sidero.dev")
)

// ClusterWorkloadProxyStatus holds the information about an exposed service for workload cluster proxying feature.
type ClusterWorkloadProxyStatus = typed.Resource[ClusterWorkloadProxyStatusSpec, ClusterWorkloadProxyStatusExtension]

// ClusterWorkloadProxyStatusSpec wraps specs.ClusterWorkloadProxyStatusSpec.
type ClusterWorkloadProxyStatusSpec = protobuf.ResourceSpec[specs.ClusterWorkloadProxyStatusSpec, *specs.ClusterWorkloadProxyStatusSpec]

// ClusterWorkloadProxyStatusExtension provides auxiliary methods for ClusterWorkloadProxyStatus resource.
type ClusterWorkloadProxyStatusExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (ClusterWorkloadProxyStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ClusterWorkloadProxyStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Num Exposed Services",
				JSONPath: "{.numexposedservices}",
			},
		},
	}
}
