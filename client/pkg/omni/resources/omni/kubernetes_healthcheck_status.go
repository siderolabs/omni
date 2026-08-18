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

// NewKubernetesHealthCheckStatus creates new KubernetesHealthCheckStatus state.
func NewKubernetesHealthCheckStatus(id string) *KubernetesHealthCheckStatus {
	return typed.NewResource[KubernetesHealthCheckStatusSpec, KubernetesHealthCheckStatusExtension](
		resource.NewMetadata(resources.EphemeralNamespace, KubernetesHealthCheckStatusType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.KubernetesHealthCheckStatusSpec{}),
	)
}

// KubernetesHealthCheckStatusType is the type of KubernetesHealthCheckStatus resource.
//
// tsgen:KubernetesHealthCheckStatusType
const KubernetesHealthCheckStatusType = resource.Type("KubernetesHealthCheckStatuses.omni.sidero.dev")

// KubernetesHealthCheckStatus resource describes the outcome of the last run of a healthcheck.
//
// KubernetesHealthCheckStatus resource ID matches the ID of the KubernetesHealthCheck it describes.
type KubernetesHealthCheckStatus = typed.Resource[KubernetesHealthCheckStatusSpec, KubernetesHealthCheckStatusExtension]

// KubernetesHealthCheckStatusSpec wraps specs.KubernetesHealthCheckStatusSpec.
type KubernetesHealthCheckStatusSpec = protobuf.ResourceSpec[specs.KubernetesHealthCheckStatusSpec, *specs.KubernetesHealthCheckStatusSpec]

// KubernetesHealthCheckStatusExtension provides auxiliary methods for KubernetesHealthCheckStatus resource.
type KubernetesHealthCheckStatusExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (KubernetesHealthCheckStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             KubernetesHealthCheckStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.EphemeralNamespace,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "State",
				JSONPath: "{.state}",
			},
			{
				Name:     "Error",
				JSONPath: "{.error}",
			},
		},
	}
}
