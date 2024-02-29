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

// NewControlPlaneStatus creates new ControlPlaneStatus resource.
func NewControlPlaneStatus(ns string, id resource.ID) *ControlPlaneStatus {
	return typed.NewResource[ControlPlaneStatusSpec, ControlPlaneStatusExtension](
		resource.NewMetadata(ns, ControlPlaneStatusType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.ControlPlaneStatusSpec{}),
	)
}

const (
	// ControlPlaneStatusType is the type of the ControlPlaneStatus resource.
	// tsgen:ControlPlaneStatusType
	ControlPlaneStatusType = resource.Type("ControlPlaneStatuses.omni.sidero.dev")
)

// ControlPlaneStatus describes control plane machine set additional status.
type ControlPlaneStatus = typed.Resource[ControlPlaneStatusSpec, ControlPlaneStatusExtension]

// ControlPlaneStatusSpec wraps specs.ControlPlaneStatusSpec.
type ControlPlaneStatusSpec = protobuf.ResourceSpec[specs.ControlPlaneStatusSpec, *specs.ControlPlaneStatusSpec]

// ControlPlaneStatusExtension provides auxiliary methods for ControlPlaneStatus resource.
type ControlPlaneStatusExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (ControlPlaneStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ControlPlaneStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
