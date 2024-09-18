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

// NewClusterDiagnostics creates new cluster machine diagnostics resource.
func NewClusterDiagnostics(ns string, id resource.ID) *ClusterDiagnostics {
	return typed.NewResource[ClusterDiagnosticsSpec, ClusterDiagnosticsExtension](
		resource.NewMetadata(ns, ClusterDiagnosticsType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.ClusterDiagnosticsSpec{}),
	)
}

const (
	// ClusterDiagnosticsType is the type of the ClusterDiagnostics resource.
	// tsgen:ClusterDiagnosticsType
	ClusterDiagnosticsType = resource.Type("ClusterDiagnostics.omni.sidero.dev")
)

// ClusterDiagnostics contains the nodes with diagnostics information.
type ClusterDiagnostics = typed.Resource[ClusterDiagnosticsSpec, ClusterDiagnosticsExtension]

// ClusterDiagnosticsSpec wraps specs.ClusterDiagnosticsSpec.
type ClusterDiagnosticsSpec = protobuf.ResourceSpec[specs.ClusterDiagnosticsSpec, *specs.ClusterDiagnosticsSpec]

// ClusterDiagnosticsExtension provides auxiliary methods for ClusterDiagnostics resource.
type ClusterDiagnosticsExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (ClusterDiagnosticsExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ClusterDiagnosticsType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
