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

// NewMachineLabels creates new cluster machine status resource.
func NewMachineLabels(ns string, id resource.ID) *MachineLabels {
	return typed.NewResource[MachineLabelsSpec, MachineLabelsExtension](
		resource.NewMetadata(ns, MachineLabelsType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.MachineLabelsSpec{}),
	)
}

const (
	// MachineLabelsType is the type of the MachineLabels resource.
	// tsgen:MachineLabelsType
	MachineLabelsType = resource.Type("MachineLabels.omni.sidero.dev")
)

// MachineLabels contains the summary for the cluster health, availability, number of nodes.
type MachineLabels = typed.Resource[MachineLabelsSpec, MachineLabelsExtension]

// MachineLabelsSpec wraps specs.MachineLabelsSpec.
type MachineLabelsSpec = protobuf.ResourceSpec[specs.MachineLabelsSpec, *specs.MachineLabelsSpec]

// MachineLabelsExtension provides auxiliary methods for MachineLabels resource.
type MachineLabelsExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (MachineLabelsExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             MachineLabelsType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
