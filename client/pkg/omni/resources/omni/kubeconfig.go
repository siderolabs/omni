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

// NewKubeconfig creates new Kubeconfig resource.
func NewKubeconfig(id resource.ID) *Kubeconfig {
	return typed.NewResource[KubeconfigSpec, KubeconfigExtension](
		resource.NewMetadata(resources.DefaultNamespace, KubeconfigType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.KubeconfigSpec{}),
	)
}

const (
	// KubeconfigType is the type of the Kubeconfig resource.
	KubeconfigType = resource.Type("Kubeconfigs.omni.sidero.dev")
)

// Kubeconfig describes client config for Kubernetes API.
type Kubeconfig = typed.Resource[KubeconfigSpec, KubeconfigExtension]

// KubeconfigSpec wraps specs.KubeconfigSpec.
type KubeconfigSpec = protobuf.ResourceSpec[specs.KubeconfigSpec, *specs.KubeconfigSpec]

// KubeconfigExtension provides auxiliary methods for Kubeconfig resource.
type KubeconfigExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (KubeconfigExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             KubeconfigType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
