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

// NewKubernetesHealthCheck creates new KubernetesHealthCheck state.
func NewKubernetesHealthCheck(id string) *KubernetesHealthCheck {
	return typed.NewResource[KubernetesHealthCheckSpec, KubernetesHealthCheckExtension](
		resource.NewMetadata(resources.DefaultNamespace, KubernetesHealthCheckType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.KubernetesHealthCheckSpec{}),
	)
}

// KubernetesHealthCheckType is the type of KubernetesHealthCheck resource.
//
// tsgen:KubernetesHealthCheckType
const KubernetesHealthCheckType = resource.Type("KubernetesHealthChecks.omni.sidero.dev")

// KubernetesHealthCheck resource describes a healthcheck.
type KubernetesHealthCheck = typed.Resource[KubernetesHealthCheckSpec, KubernetesHealthCheckExtension]

// KubernetesHealthCheckSpec wraps specs.KubernetesHealthCheckSpec.
type KubernetesHealthCheckSpec = protobuf.ResourceSpec[specs.KubernetesHealthCheckSpec, *specs.KubernetesHealthCheckSpec]

// KubernetesHealthCheckExtension provides auxiliary methods for KubernetesHealthCheck resource.
type KubernetesHealthCheckExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (KubernetesHealthCheckExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             KubernetesHealthCheckType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
