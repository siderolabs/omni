// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package auth

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
)

const (
	// RoleBindingType is the type of RoleBinding resource.
	//
	// tsgen:RoleBindingType
	RoleBindingType = resource.Type("RoleBindings.omni.sidero.dev")
)

// NewRoleBinding creates new RoleBinding resource.
func NewRoleBinding(id string) *RoleBinding {
	return typed.NewResource[RoleBindingSpec, RoleBindingExtension](
		resource.NewMetadata(resources.DefaultNamespace, RoleBindingType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.RoleBindingSpec{}),
	)
}

// RoleBinding resource describes an RBAC role binding.
type RoleBinding = typed.Resource[RoleBindingSpec, RoleBindingExtension]

// RoleBindingSpec wraps specs.RoleBindingSpec.
type RoleBindingSpec = protobuf.ResourceSpec[specs.RoleBindingSpec, *specs.RoleBindingSpec]

// RoleBindingExtension provides auxiliary methods for RoleBinding resource.
type RoleBindingExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (RoleBindingExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             RoleBindingType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
