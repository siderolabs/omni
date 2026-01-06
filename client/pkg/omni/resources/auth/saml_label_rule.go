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

// NewSAMLLabelRule creates a new SAMLLabelRule resource.
func NewSAMLLabelRule(id string) *SAMLLabelRule {
	return typed.NewResource[SAMLLabelRuleSpec, SAMLLabelRuleExtension](
		resource.NewMetadata(resources.DefaultNamespace, SAMLLabelRuleType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.SAMLLabelRuleSpec{}),
	)
}

const (
	// SAMLLabelRuleType is the type of SAMLLabelRule resource.
	//
	// tsgen:SAMLLabelRuleType
	SAMLLabelRuleType = resource.Type("SAMLLabelRules.omni.sidero.dev")
)

// SAMLLabelRule resource describes a SAML label rule.
type SAMLLabelRule = typed.Resource[SAMLLabelRuleSpec, SAMLLabelRuleExtension]

// SAMLLabelRuleSpec wraps specs.SAMLLabelRuleSpec.
type SAMLLabelRuleSpec = protobuf.ResourceSpec[specs.SAMLLabelRuleSpec, *specs.SAMLLabelRuleSpec]

// SAMLLabelRuleExtension providers auxiliary methods for SAMLLabelRule resource.
type SAMLLabelRuleExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (SAMLLabelRuleExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             SAMLLabelRuleType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Role On Registration",
				JSONPath: "{.assignroleonregistration}",
			},
			{
				Name:     "Match Labels",
				JSONPath: "{.matchlabels}",
			},
		},
	}
}
