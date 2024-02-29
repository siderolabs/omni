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

// NewEtcdAuditResult creates new etcd audit result.
func NewEtcdAuditResult(ns string, id resource.ID) *EtcdAuditResult {
	return typed.NewResource[EtcdAuditResultSpec, EtcdAuditResultExtension](
		resource.NewMetadata(ns, EtcdAuditResultType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.EtcdAuditResultSpec{}),
	)
}

// EtcdAuditResultType is the type of the EtcdAuditResult resource.
const EtcdAuditResultType = resource.Type("EtcdAuditResults.omni.sidero.dev")

// EtcdAuditResult describes etcd backup encryption data.
type EtcdAuditResult = typed.Resource[EtcdAuditResultSpec, EtcdAuditResultExtension]

// EtcdAuditResultSpec wraps specs.EtcdAuditResultSpec.
type EtcdAuditResultSpec = protobuf.ResourceSpec[specs.EtcdAuditResultSpec, *specs.EtcdAuditResultSpec]

// EtcdAuditResultExtension provides auxiliary methods for EtcdAuditResult resource.
type EtcdAuditResultExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (EtcdAuditResultExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             EtcdAuditResultType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
