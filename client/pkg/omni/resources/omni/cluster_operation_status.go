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

// NewClusterOperationStatus creates new ClusterOperationStatus resource.
func NewClusterOperationStatus(ns string, id resource.ID) *ClusterOperationStatus {
	return typed.NewResource[ClusterOperationStatusSpec, ClusterOperationStatusExtension](
		resource.NewMetadata(ns, ClusterOperationStatusType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.ClusterOperationStatusSpec{}),
	)
}

const (
	// ClusterOperationStatusType is the type of the ClusterOperationStatus resource.
	// tsgen:ClusterOperationStatusType
	ClusterOperationStatusType = resource.Type("ClusterOperationStatuses.omni.sidero.dev")
)

// ClusterOperationStatus describes control plane machine set additional status.
type ClusterOperationStatus = typed.Resource[ClusterOperationStatusSpec, ClusterOperationStatusExtension]

// ClusterOperationStatusSpec wraps specs.ClusterOperationStatusSpec.
type ClusterOperationStatusSpec = protobuf.ResourceSpec[specs.ClusterOperationStatusSpec, *specs.ClusterOperationStatusSpec]

// ClusterOperationStatusExtension provides auxiliary methods for ClusterOperationStatus resource.
type ClusterOperationStatusExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (ClusterOperationStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ClusterOperationStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
