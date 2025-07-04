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

// NewMachineConfigDiff creates a new MachineConfigDiff resource.
func NewMachineConfigDiff(id resource.ID) *MachineConfigDiff {
	return typed.NewResource[MachineConfigDiffSpec, MachineConfigDiffExtension](
		resource.NewMetadata(resources.DefaultNamespace, MachineConfigDiffType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.MachineConfigDiffSpec{}),
	)
}

const (
	// MachineConfigDiffType is the type of the MachineConfigDiff resource.
	// tsgen:MachineConfigDiffType
	MachineConfigDiffType = resource.Type("MachineConfigDiffs.omni.sidero.dev")
)

// MachineConfigDiff is the diff between two redacted machine configurations.
type MachineConfigDiff = typed.Resource[MachineConfigDiffSpec, MachineConfigDiffExtension]

// MachineConfigDiffSpec wraps specs.MachineConfigDiffSpec.
type MachineConfigDiffSpec = protobuf.ResourceSpec[specs.MachineConfigDiffSpec, *specs.MachineConfigDiffSpec]

// MachineConfigDiffExtension provides auxiliary methods for MachineConfigDiff resource.
type MachineConfigDiffExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (MachineConfigDiffExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             MachineConfigDiffType,
		DefaultNamespace: resources.DefaultNamespace,
	}
}
