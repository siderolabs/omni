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

// NewClusterMetrics creates a new ClusterMetrics resource.
func NewClusterMetrics(ns string, id resource.ID) *ClusterMetrics {
	return typed.NewResource[ClusterMetricsSpec, ClusterMetricsExtension](
		resource.NewMetadata(ns, ClusterMetricsType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.ClusterMetricsSpec{}),
	)
}

const (
	// ClusterMetricsType is the type of the ClusterMetrics resource.
	// tsgen:ClusterMetricsType
	ClusterMetricsType = resource.Type("ClusterMetrics.omni.sidero.dev")

	// ClusterMetricsID is the ID of the single resource for the machine status metrics resource.
	// tsgen:ClusterMetricsID
	ClusterMetricsID = "metrics"
)

// ClusterMetrics describes cluster status metrics resource.
type ClusterMetrics = typed.Resource[ClusterMetricsSpec, ClusterMetricsExtension]

// ClusterMetricsSpec wraps specs.ClusterMetricsSpec.
type ClusterMetricsSpec = protobuf.ResourceSpec[specs.ClusterMetricsSpec, *specs.ClusterMetricsSpec]

// ClusterMetricsExtension provides auxiliary methods for ClusterMetrics resource.
type ClusterMetricsExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (ClusterMetricsExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ClusterMetricsType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.EphemeralNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
