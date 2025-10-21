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

// NewKernelArgsStatus creates new KernelArgsStatus resource.
func NewKernelArgsStatus(id resource.ID) *KernelArgsStatus {
	return typed.NewResource[KernelArgsStatusSpec, KernelArgsStatusExtension](
		resource.NewMetadata(resources.DefaultNamespace, KernelArgsStatusType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.KernelArgsStatusSpec{}),
	)
}

const (
	// KernelArgsStatusType is the type of the KernelArgsStatus resource.
	// tsgen:KernelArgsStatusType
	KernelArgsStatusType = resource.Type("KernelArgsStatuses.omni.sidero.dev")
)

// KernelArgsStatus describes the desired machine KernelArgsStatus for the machine with the same ID.
type KernelArgsStatus = typed.Resource[KernelArgsStatusSpec, KernelArgsStatusExtension]

// KernelArgsStatusSpec wraps specs.KernelArgsStatusSpec.
type KernelArgsStatusSpec = protobuf.ResourceSpec[specs.KernelArgsStatusSpec, *specs.KernelArgsStatusSpec]

// KernelArgsStatusExtension provides auxiliary methods for KernelArgsStatus resource.
type KernelArgsStatusExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (KernelArgsStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             KernelArgsStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Args",
				JSONPath: "{.args}",
			},
			{
				Name:     "Current Args",
				JSONPath: "{.currentargs}",
			},
			{
				Name:     "Unmet Conditions",
				JSONPath: "{.unmetconditions}",
			},
			{
				Name:     "Current Cmdline",
				JSONPath: "{.currentcmdline}",
			},
		},
	}
}
