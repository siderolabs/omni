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
	"github.com/siderolabs/omni/client/pkg/omni/resources"
)

// NewJoinTokenStatus creates a new JoinTokenStatus resource.
func NewJoinTokenStatus(ns, id string) *JoinTokenStatus {
	return typed.NewResource[JoinTokenStatusSpec, JoinTokenStatusExtension](
		resource.NewMetadata(ns, JoinTokenStatusType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.JoinTokenStatusSpec{}),
	)
}

const (
	// JoinTokenStatusType is the type of JoinTokenStatus resource.
	//
	// tsgen:JoinTokenStatusType
	JoinTokenStatusType = resource.Type("JoinTokenStatuses.omni.sidero.dev")
)

// JoinTokenStatus resource keeps the available join tokens that Talos nodes can use for joining Omni.
type JoinTokenStatus = typed.Resource[JoinTokenStatusSpec, JoinTokenStatusExtension]

// JoinTokenStatusSpec wraps specs.JoinTokenStatusSpec.
type JoinTokenStatusSpec = protobuf.ResourceSpec[specs.JoinTokenStatusSpec, *specs.JoinTokenStatusSpec]

// JoinTokenStatusExtension providers auxiliary methods for JoinTokenStatus resource.
type JoinTokenStatusExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (JoinTokenStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             JoinTokenStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
