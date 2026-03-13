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

// NewClusterKubernetesManifestsStatus creates new cluster kubernetes manifest status resource.
func NewClusterKubernetesManifestsStatus(id resource.ID) *ClusterKubernetesManifestsStatus {
	return typed.NewResource[ClusterKubernetesManifestsStatusSpec, ClusterKubernetesManifestsStatusExtension](
		resource.NewMetadata(resources.DefaultNamespace, ClusterKubernetesManifestsStatusType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.ClusterKubernetesManifestsStatusSpec{}),
	)
}

const (
	// ClusterKubernetesManifestsStatusType is the type of the ClusterKubernetesManifestsStatus resource.
	// tsgen:ClusterKubernetesManifestsStatusType
	ClusterKubernetesManifestsStatusType = resource.Type("ClusterKubernetesManifestsStatuses.omni.sidero.dev")
)

// ClusterKubernetesManifestsStatus is the status of the KubernetesManifest.
type ClusterKubernetesManifestsStatus = typed.Resource[ClusterKubernetesManifestsStatusSpec, ClusterKubernetesManifestsStatusExtension]

// ClusterKubernetesManifestsStatusSpec wraps specs.ClusterKubernetesManifestsStatusSpec.
type ClusterKubernetesManifestsStatusSpec = protobuf.ResourceSpec[specs.ClusterKubernetesManifestsStatusSpec, *specs.ClusterKubernetesManifestsStatusSpec]

// ClusterKubernetesManifestsStatusExtension provides auxiliary methods for ClusterKubernetesManifestsStatus resource.
type ClusterKubernetesManifestsStatusExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (ClusterKubernetesManifestsStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ClusterKubernetesManifestsStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Out Of Sync",
				JSONPath: "{.outofsync}",
			},
			{
				Name:     "Total Manifests",
				JSONPath: "{.total}",
			},
		},
	}
}
