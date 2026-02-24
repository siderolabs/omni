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

// NewUpgradeRollout creates a new UpgradeRollout resource.
func NewUpgradeRollout(id resource.ID) *UpgradeRollout {
	return typed.NewResource[UpgradeRolloutSpec, UpgradeRolloutExtension](
		resource.NewMetadata(resources.EphemeralNamespace, UpgradeRolloutType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.UpgradeRolloutSpec{}),
	)
}

// UpgradeRolloutType is the type of UpgradeRollout resource.
//
// tsgen:UpgradeRolloutType
const UpgradeRolloutType = resource.Type("UpgradeRollouts.omni.sidero.dev")

// UpgradeRollout resource describes upgrade locks.
//
// UpgradeRollout resource ID is a cluster ID.
type UpgradeRollout = typed.Resource[UpgradeRolloutSpec, UpgradeRolloutExtension]

// UpgradeRolloutSpec wraps specs.UpgradeRolloutSpec.
type UpgradeRolloutSpec = protobuf.ResourceSpec[specs.UpgradeRolloutSpec, *specs.UpgradeRolloutSpec]

// UpgradeRolloutExtension providers auxiliary methods for UpgradeRollout resource.
type UpgradeRolloutExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (UpgradeRolloutExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             UpgradeRolloutType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.EphemeralNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
