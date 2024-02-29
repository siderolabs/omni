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
	// AccessPolicyID is the ID of AccessPolicy resource.
	AccessPolicyID = "access-policy"

	// AccessPolicyType is the type of AccessPolicy resource.
	//
	// tsgen:AccessPolicyType
	AccessPolicyType = resource.Type("AccessPolicies.omni.sidero.dev")
)

// NewAccessPolicy creates new AccessPolicy resource.
func NewAccessPolicy() *AccessPolicy {
	return typed.NewResource[AccessPolicySpec, AccessPolicyExtension](
		resource.NewMetadata(resources.DefaultNamespace, AccessPolicyType, AccessPolicyID, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.AccessPolicySpec{}),
	)
}

// AccessPolicy resource describes a user ACL.
type AccessPolicy = typed.Resource[AccessPolicySpec, AccessPolicyExtension]

// AccessPolicySpec wraps specs.AccessPolicySpec.
type AccessPolicySpec = protobuf.ResourceSpec[specs.AccessPolicySpec, *specs.AccessPolicySpec]

// AccessPolicyExtension providers auxiliary methods for AccessPolicy resource.
type AccessPolicyExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (AccessPolicyExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             AccessPolicyType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
