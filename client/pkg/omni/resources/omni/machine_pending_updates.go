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

// NewMachinePendingUpdates creates new MachinePendingUpdates resource.
func NewMachinePendingUpdates(id resource.ID) *MachinePendingUpdates {
	return typed.NewResource[MachinePendingUpdatesSpec, MachinePendingUpdatesExtension](
		resource.NewMetadata(resources.DefaultNamespace, MachinePendingUpdatesType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.MachinePendingUpdatesSpec{}),
	)
}

const (
	// MachinePendingUpdatesType is the type of the MachinePendingUpdates resource.
	// tsgen:MachinePendingUpdatesType
	MachinePendingUpdatesType = resource.Type("MachinePendingUpdates.omni.sidero.dev")
)

// MachinePendingUpdates shows the currently pending changes in the machine config, talos version and schematic.
type MachinePendingUpdates = typed.Resource[MachinePendingUpdatesSpec, MachinePendingUpdatesExtension]

// MachinePendingUpdatesSpec wraps specs.MachinePendingUpdatesSpec.
type MachinePendingUpdatesSpec = protobuf.ResourceSpec[specs.MachinePendingUpdatesSpec, *specs.MachinePendingUpdatesSpec]

// MachinePendingUpdatesExtension provides auxiliary methods for MachinePendingUpdates resource.
type MachinePendingUpdatesExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (MachinePendingUpdatesExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             MachinePendingUpdatesType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
