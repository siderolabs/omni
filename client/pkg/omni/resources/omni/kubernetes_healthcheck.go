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

// NewKubernetesHealthcheck creates new KubernetesHealthcheck state.
func NewKubernetesHealthcheck(id string) *KubernetesHealthcheck {
	return typed.NewResource[KubernetesHealthcheckSpec, KubernetesHealthcheckExtension](
		resource.NewMetadata(resources.DefaultNamespace, KubernetesHealthcheckType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.KubernetesHealthcheckSpec{}),
	)
}

// KubernetesHealthcheckType is the type of KubernetesHealthcheck resource.
//
// tsgen:KubernetesHealthcheckType
const KubernetesHealthcheckType = resource.Type("KubernetesHealthchecks.omni.sidero.dev")

// KubernetesHealthcheck resource describes a healthcheck.
type KubernetesHealthcheck = typed.Resource[KubernetesHealthcheckSpec, KubernetesHealthcheckExtension]

// KubernetesHealthcheckSpec wraps specs.KubernetesHealthcheckSpec.
type KubernetesHealthcheckSpec = protobuf.ResourceSpec[specs.KubernetesHealthcheckSpec, *specs.KubernetesHealthcheckSpec]

// KubernetesHealthcheckExtension provides auxiliary methods for KubernetesHealthcheck resource.
type KubernetesHealthcheckExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (KubernetesHealthcheckExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             KubernetesHealthcheckType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
