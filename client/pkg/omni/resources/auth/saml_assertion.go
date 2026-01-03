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

// NewSAMLAssertion creates a new SAMLAssertion resource.
func NewSAMLAssertion(id string) *SAMLAssertion {
	return typed.NewResource[SAMLAssertionSpec, SAMLAssertionExtension](
		resource.NewMetadata(resources.DefaultNamespace, SAMLAssertionType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.SAMLAssertionSpec{}),
	)
}

const (
	// SAMLAssertionType is the type of SAMLAssertion resource.
	SAMLAssertionType = resource.Type("SAMLAssertions.omni.sidero.dev")
)

// SAMLAssertion resource describes SAML assertion.
type SAMLAssertion = typed.Resource[SAMLAssertionSpec, SAMLAssertionExtension]

// SAMLAssertionSpec wraps specs.SAMLAssertionSpec.
type SAMLAssertionSpec = protobuf.ResourceSpec[specs.SAMLAssertionSpec, *specs.SAMLAssertionSpec]

// SAMLAssertionExtension providers auxiliary methods for SAMLAssertion resource.
type SAMLAssertionExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (SAMLAssertionExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             SAMLAssertionType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
