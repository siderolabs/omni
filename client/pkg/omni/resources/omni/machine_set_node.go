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

// NewMachineSetNode creates new MachineSetNode resource.
func NewMachineSetNode(ns string, id resource.ID, owner resource.Resource) *MachineSetNode {
	// this should never happen, simple sanity check
	if owner.Metadata().Type() != MachineSetType {
		panic("the owner of a machine set node must always be a machine set")
	}

	res := typed.NewResource[MachineSetNodeSpec, MachineSetNodeExtension](
		resource.NewMetadata(ns, MachineSetNodeType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.MachineSetNodeSpec{}),
	)

	for _, label := range []string{
		LabelCluster,
		LabelControlPlaneRole,
		LabelWorkerRole,
	} {
		val, ok := owner.Metadata().Labels().Get(label)
		if ok {
			res.Metadata().Labels().Set(label, val)
		}
	}

	res.Metadata().Labels().Set(LabelMachineSet, owner.Metadata().ID())

	return res
}

const (
	// MachineSetNodeType is the type of the MachineSetNode resource.
	// tsgen:MachineSetNodeType
	MachineSetNodeType = resource.Type("MachineSetNodes.omni.sidero.dev")
)

// MachineSetNode describes machine set node resource.
type MachineSetNode = typed.Resource[MachineSetNodeSpec, MachineSetNodeExtension]

// MachineSetNodeSpec wraps specs.MachineSetNodeSpec.
type MachineSetNodeSpec = protobuf.ResourceSpec[specs.MachineSetNodeSpec, *specs.MachineSetNodeSpec]

// MachineSetNodeExtension provides auxiliary methods for MachineSetNode resource.
type MachineSetNodeExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (MachineSetNodeExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             MachineSetNodeType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
