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

// NewClusterDestroyStatus creates new cluster destroy status.
func NewClusterDestroyStatus(id resource.ID) *ClusterDestroyStatus {
	return typed.NewResource[ClusterDestroyStatusSpec, ClusterDestroyStatusExtension](
		resource.NewMetadata(resources.DefaultNamespace, ClusterDestroyStatusType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.DestroyStatusSpec{}),
	)
}

const (
	// ClusterDestroyStatusType is the type of the ClusterDestroyStatus resource.
	// tsgen:ClusterDestroyStatusType
	ClusterDestroyStatusType = resource.Type("ClusterDestroyStatuses.omni.sidero.dev")
)

// ClusterDestroyStatus contains the state of cluster destroy for clusters in TearingDown phase.
type ClusterDestroyStatus = typed.Resource[ClusterDestroyStatusSpec, ClusterDestroyStatusExtension]

// ClusterDestroyStatusSpec wraps specs.ClusterDestroyStatusSpec.
type ClusterDestroyStatusSpec = protobuf.ResourceSpec[specs.DestroyStatusSpec, *specs.DestroyStatusSpec]

// ClusterDestroyStatusExtension provides auxiliary methods for ClusterDestroyStatus resource.
type ClusterDestroyStatusExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (ClusterDestroyStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ClusterDestroyStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Phase",
				JSONPath: "{.phase}",
			},
		},
	}
}
