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

// NewDiscoveryAuditResult creates a new discovery service audit result.
func NewDiscoveryAuditResult(ns string, id resource.ID) *DiscoveryAuditResult {
	return typed.NewResource[DiscoveryAuditResultSpec, DiscoveryAuditResultExtension](
		resource.NewMetadata(ns, DiscoveryAuditResultType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.DiscoveryAuditResultSpec{}),
	)
}

// DiscoveryAuditResultType is the type of the DiscoveryAuditResult resource.
const DiscoveryAuditResultType = resource.Type("DiscoveryAuditResults.omni.sidero.dev")

// DiscoveryAuditResult describes the discovery service audit result.
type DiscoveryAuditResult = typed.Resource[DiscoveryAuditResultSpec, DiscoveryAuditResultExtension]

// DiscoveryAuditResultSpec wraps specs.DiscoveryAuditResultSpec.
type DiscoveryAuditResultSpec = protobuf.ResourceSpec[specs.DiscoveryAuditResultSpec, *specs.DiscoveryAuditResultSpec]

// DiscoveryAuditResultExtension provides auxiliary methods for DiscoveryAuditResult resource.
type DiscoveryAuditResultExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (DiscoveryAuditResultExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             DiscoveryAuditResultType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Cluster ID",
				JSONPath: "{.clusterid}",
			},
			{
				Name:     "Affiliate IDs",
				JSONPath: "{.deletedaffiliateids}",
			},
		},
	}
}
