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

// NewMachineStatusMetrics creates new MachineStatusMetrics resource.
func NewMachineStatusMetrics(id resource.ID) *MachineStatusMetrics {
	return typed.NewResource[MachineStatusMetricsSpec, MachineStatusMetricsExtension](
		resource.NewMetadata(resources.EphemeralNamespace, MachineStatusMetricsType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.MachineStatusMetricsSpec{}),
	)
}

const (
	// MachineStatusMetricsType is the type of the MachineStatusMetrics resource.
	// tsgen:MachineStatusMetricsType
	MachineStatusMetricsType = resource.Type("MachineStatusMetrics.omni.sidero.dev")

	// MachineStatusMetricsID is the ID of the single resource for the machine status metrics resource.
	// tsgen:MachineStatusMetricsID
	MachineStatusMetricsID = "metrics"
)

// MachineStatusMetrics describes machine status metrics resource.
type MachineStatusMetrics = typed.Resource[MachineStatusMetricsSpec, MachineStatusMetricsExtension]

// MachineStatusMetricsSpec wraps specs.MachineStatusMetricsSpec.
type MachineStatusMetricsSpec = protobuf.ResourceSpec[specs.MachineStatusMetricsSpec, *specs.MachineStatusMetricsSpec]

// MachineStatusMetricsExtension provides auxiliary methods for MachineStatusMetrics resource.
type MachineStatusMetricsExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (MachineStatusMetricsExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             MachineStatusMetricsType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.EphemeralNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
