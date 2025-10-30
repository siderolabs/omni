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

// NewKernelArgs creates new KernelArgs resource.
func NewKernelArgs(id resource.ID) *KernelArgs {
	return typed.NewResource[KernelArgsSpec, KernelArgsExtension](
		resource.NewMetadata(resources.DefaultNamespace, KernelArgsType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.KernelArgsSpec{}),
	)
}

const (
	// KernelArgsType is the type of the KernelArgs resource.
	// tsgen:KernelArgsType
	KernelArgsType = resource.Type("KernelArgs.omni.sidero.dev")
)

// KernelArgs describes the desired machine KernelArgs for the machine with the same ID.
type KernelArgs = typed.Resource[KernelArgsSpec, KernelArgsExtension]

// KernelArgsSpec wraps specs.KernelArgsSpec.
type KernelArgsSpec = protobuf.ResourceSpec[specs.KernelArgsSpec, *specs.KernelArgsSpec]

// KernelArgsExtension provides auxiliary methods for KernelArgs resource.
type KernelArgsExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (KernelArgsExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             KernelArgsType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Args",
				JSONPath: "{.args}",
			},
		},
	}
}
