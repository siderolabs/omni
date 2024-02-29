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

// NewTalosUpgradeStatus creates new LoadBalancer state.
func NewTalosUpgradeStatus(ns, id string) *TalosUpgradeStatus {
	return typed.NewResource[TalosUpgradeStatusSpec, TalosUpgradeStatusExtension](
		resource.NewMetadata(ns, TalosUpgradeStatusType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.TalosUpgradeStatusSpec{}),
	)
}

// TalosUpgradeStatusType is a resource type that contains the current state of the machine set Talos upgrade.
//
// tsgen:TalosUpgradeStatusType
const TalosUpgradeStatusType = resource.Type("TalosUpgradeStatuses.omni.sidero.dev")

// TalosUpgradeStatus is a resource type that contains the configuration of a load balancer.
type TalosUpgradeStatus = typed.Resource[TalosUpgradeStatusSpec, TalosUpgradeStatusExtension]

// TalosUpgradeStatusSpec wraps specs.TalosUpgradeStatusSpec.
type TalosUpgradeStatusSpec = protobuf.ResourceSpec[specs.TalosUpgradeStatusSpec, *specs.TalosUpgradeStatusSpec]

// TalosUpgradeStatusExtension providers auxiliary methods for TalosUpgradeStatus resource.
type TalosUpgradeStatusExtension struct{}

// ResourceDefinition implements typed.ResourceDefinition interface.
func (TalosUpgradeStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             TalosUpgradeStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
