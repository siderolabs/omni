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

// NewIdentityLabelRule creates a new IdentityLabelRule resource.
func NewIdentityLabelRule(id string) *IdentityLabelRule {
	return typed.NewResource[IdentityLabelRuleSpec, IdentityLabelRuleExtension](
		resource.NewMetadata(resources.DefaultNamespace, IdentityLabelRuleType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.IdentityLabelRuleSpec{}),
	)
}

const (
	// IdentityLabelRuleType is the type of IdentityLabelRule resource.
	//
	// tsgen:IdentityLabelRuleType
	IdentityLabelRuleType = resource.Type("IdentityLabelRules.omni.sidero.dev")
)

// IdentityLabelRule resource describes a rule which assigns an Omni role to the identities matching its labels.
type IdentityLabelRule = typed.Resource[IdentityLabelRuleSpec, IdentityLabelRuleExtension]

// IdentityLabelRuleSpec wraps specs.IdentityLabelRuleSpec.
type IdentityLabelRuleSpec = protobuf.ResourceSpec[specs.IdentityLabelRuleSpec, *specs.IdentityLabelRuleSpec]

// IdentityLabelRuleExtension provides auxiliary methods for IdentityLabelRule resource.
type IdentityLabelRuleExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (IdentityLabelRuleExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             IdentityLabelRuleType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Role",
				JSONPath: "{.assignrole}",
			},
			{
				Name:     "Match Labels",
				JSONPath: "{.matchlabels}",
			},
		},
	}
}
