// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package siderolink

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/omni/client/api/omni/specs"
)

// NOTE: This resources is not used anymore, but still used in the migration code.

// NewDeprecatedLinkCounter creates new LinkCounter state.
func NewDeprecatedLinkCounter(ns, id string) *DeprecatedLinkCounter {
	return typed.NewResource[DeprecatedLinkCounterSpec, DeprecatedLinkCounterExtension](
		resource.NewMetadata(ns, DeprecatedLinkCounterType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.SiderolinkCounterSpec{}),
	)
}

// DeprecatedLinkCounterType is the type of LinkCounter resource.
const DeprecatedLinkCounterType = resource.Type("LinkCounters.omni.sidero.dev")

// DeprecatedLinkCounter resource was removed, but still used only in the migration code.
type DeprecatedLinkCounter = typed.Resource[DeprecatedLinkCounterSpec, DeprecatedLinkCounterExtension]

// DeprecatedLinkCounterSpec wraps specs.SiderolinkSpec.
type DeprecatedLinkCounterSpec = protobuf.ResourceSpec[specs.SiderolinkCounterSpec, *specs.SiderolinkCounterSpec]

// DeprecatedLinkCounterExtension providers auxiliary methods for LinkCounter resource.
type DeprecatedLinkCounterExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (DeprecatedLinkCounterExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             DeprecatedLinkCounterType,
		Aliases:          []resource.Type{},
		DefaultNamespace: CounterNamespace,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "RX",
				JSONPath: "{.bytesreceived}",
			},
			{
				Name:     "TX",
				JSONPath: "{.bytessent}",
			},
		},
	}
}
