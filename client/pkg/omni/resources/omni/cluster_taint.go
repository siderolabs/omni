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

// NewClusterTaint creates new cluster taint resource.
func NewClusterTaint(ns string, id resource.ID) *ClusterTaint {
	return typed.NewResource[ClusterTaintSpec, ClusterTaintExtension](
		resource.NewMetadata(ns, ClusterTaintType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.ClusterTaintSpec{}),
	)
}

const (
	// ClusterTaintType is the type of the ClusterTaint resource.
	// tsgen:ClusterTaintType
	ClusterTaintType = resource.Type("ClusterTaints.omni.sidero.dev")
)

// ClusterTaint signals that Talos or Kubernetes configs are no longer under full control of Omni
// as break-glass configs that bypass Omni were generated.
type ClusterTaint = typed.Resource[ClusterTaintSpec, ClusterTaintExtension]

// ClusterTaintSpec wraps specs.ClusterTaintSpec.
type ClusterTaintSpec = protobuf.ResourceSpec[specs.ClusterTaintSpec, *specs.ClusterTaintSpec]

// ClusterTaintExtension provides auxiliary methods for ClusterTaint resource.
type ClusterTaintExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (ClusterTaintExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ClusterTaintType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
