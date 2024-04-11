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

// NewKubernetesNodeAuditResult creates a new Kubernetes node audit result.
func NewKubernetesNodeAuditResult(ns string, id resource.ID) *KubernetesNodeAuditResult {
	return typed.NewResource[KubernetesNodeAuditResultSpec, KubernetesNodeAuditResultExtension](
		resource.NewMetadata(ns, KubernetesNodeAuditResultType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.KubernetesNodeAuditResultSpec{}),
	)
}

// KubernetesNodeAuditResultType is the type of the KubernetesNodeAuditResult resource.
const KubernetesNodeAuditResultType = resource.Type("KubernetesNodeAuditResults.omni.sidero.dev")

// KubernetesNodeAuditResult describes the Kubernetes node audit result.
type KubernetesNodeAuditResult = typed.Resource[KubernetesNodeAuditResultSpec, KubernetesNodeAuditResultExtension]

// KubernetesNodeAuditResultSpec wraps specs.KubernetesNodeAuditResultSpec.
type KubernetesNodeAuditResultSpec = protobuf.ResourceSpec[specs.KubernetesNodeAuditResultSpec, *specs.KubernetesNodeAuditResultSpec]

// KubernetesNodeAuditResultExtension provides auxiliary methods for KubernetesNodeAuditResult resource.
type KubernetesNodeAuditResultExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (KubernetesNodeAuditResultExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             KubernetesNodeAuditResultType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Nodes",
				JSONPath: "{.nodes}",
			},
		},
	}
}
