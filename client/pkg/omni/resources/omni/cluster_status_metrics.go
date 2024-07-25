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

// NewClusterStatusMetrics creates new ClusterStatusMetrics resource.
func NewClusterStatusMetrics(ns string, id resource.ID) *ClusterStatusMetrics {
	return typed.NewResource[ClusterStatusMetricsSpec, ClusterStatusMetricsExtension](
		resource.NewMetadata(ns, ClusterStatusMetricsType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.ClusterStatusMetricsSpec{}),
	)
}

const (
	// ClusterStatusMetricsType is the type of the ClusterStatusMetrics resource.
	// tsgen:ClusterStatusMetricsType
	ClusterStatusMetricsType = resource.Type("ClusterStatusMetrics.omni.sidero.dev")

	// ClusterStatusMetricsID is the ID of the single resource for the machine status metrics resource.
	// tsgen:ClusterStatusMetricsID
	ClusterStatusMetricsID = "metrics"
)

// ClusterStatusMetrics describes cluster status metrics resource.
type ClusterStatusMetrics = typed.Resource[ClusterStatusMetricsSpec, ClusterStatusMetricsExtension]

// ClusterStatusMetricsSpec wraps specs.ClusterStatusMetricsSpec.
type ClusterStatusMetricsSpec = protobuf.ResourceSpec[specs.ClusterStatusMetricsSpec, *specs.ClusterStatusMetricsSpec]

// ClusterStatusMetricsExtension provides auxiliary methods for ClusterStatusMetrics resource.
type ClusterStatusMetricsExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (ClusterStatusMetricsExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ClusterStatusMetricsType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.EphemeralNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
