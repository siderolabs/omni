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

// NewDryRunStatus creates new DryRunStatus resource.
func NewDryRunStatus(id resource.ID) *DryRunStatus {
	return typed.NewResource[DryRunStatusSpec, DryRunStatusExtension](
		resource.NewMetadata(resources.DefaultNamespace, DryRunStatusType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.DryRunStatusSpec{}),
	)
}

const (
	// DryRunStatusType is the type of the DryRunStatus resource.
	// tsgen:DryRunStatusType
	DryRunStatusType = resource.Type("DryRunStatuses.omni.sidero.dev")
)

// DryRunStatus is the result of the Talos dry run of the pending config of a machine in dry-run mode.
type DryRunStatus = typed.Resource[DryRunStatusSpec, DryRunStatusExtension]

// DryRunStatusSpec wraps specs.DryRunStatusSpec.
type DryRunStatusSpec = protobuf.ResourceSpec[specs.DryRunStatusSpec, *specs.DryRunStatusSpec]

// DryRunStatusExtension provides auxiliary methods for DryRunStatus resource.
type DryRunStatusExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (DryRunStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             DryRunStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
