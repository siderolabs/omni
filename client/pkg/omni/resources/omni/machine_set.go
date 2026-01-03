// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package omni

import (
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
)

// ControlPlanesIDSuffix is the suffix for the MachineSet control planes of a cluster.
//
// tsgen:ControlPlanesIDSuffix
const ControlPlanesIDSuffix = "control-planes"

// DefaultWorkersIDSuffix is the suffix for the MachineSet workers of a cluster.
//
// tsgen:DefaultWorkersIDSuffix
const DefaultWorkersIDSuffix = "workers"

// ControlPlanesResourceID returns the name for the MachineSet control planes of a cluster.
func ControlPlanesResourceID(clusterName string) string {
	return fmt.Sprintf("%s-%s", clusterName, ControlPlanesIDSuffix)
}

// WorkersResourceID returns the name for the default MachineSet workers of a cluster.
func WorkersResourceID(clusterName string) string {
	return fmt.Sprintf("%s-%s", clusterName, DefaultWorkersIDSuffix)
}

// AdditionalWorkersResourceID returns the name for the additional MachineSet workers of a cluster.
func AdditionalWorkersResourceID(clusterName, name string) string {
	return fmt.Sprintf("%s-%s", clusterName, name)
}

// NewMachineSet creates new MachineSet resource.
func NewMachineSet(id resource.ID) *MachineSet {
	return typed.NewResource[MachineSetSpec, MachineSetExtension](
		resource.NewMetadata(resources.DefaultNamespace, MachineSetType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.MachineSetSpec{}),
	)
}

const (
	// MachineSetType is the type of the MachineSet resource.
	// tsgen:MachineSetType
	MachineSetType = resource.Type("MachineSets.omni.sidero.dev")
)

// MachineSet describes machine set resource.
type MachineSet = typed.Resource[MachineSetSpec, MachineSetExtension]

// MachineSetSpec wraps specs.MachineSetSpec.
type MachineSetSpec = protobuf.ResourceSpec[specs.MachineSetSpec, *specs.MachineSetSpec]

// MachineSetExtension provides auxiliary methods for MachineSet resource.
type MachineSetExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (MachineSetExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             MachineSetType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}

// GetMachineAllocation from the machine set resource.
func GetMachineAllocation(res *MachineSet) *specs.MachineSetSpec_MachineAllocation {
	if res.TypedSpec().Value.MachineClass != nil { //nolint:staticcheck
		return res.TypedSpec().Value.MachineClass //nolint:staticcheck
	}

	if res.TypedSpec().Value.MachineAllocation != nil {
		return res.TypedSpec().Value.MachineAllocation
	}

	return nil
}
