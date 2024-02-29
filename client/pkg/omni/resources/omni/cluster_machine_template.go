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

// NewClusterMachineTemplate creates new cluster machine status resource.
func NewClusterMachineTemplate(ns string, id resource.ID) *ClusterMachineTemplate {
	return typed.NewResource[ClusterMachineTemplateSpec, ClusterMachineTemplateExtension](
		resource.NewMetadata(ns, ClusterMachineTemplateType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.ClusterMachineTemplateSpec{}),
	)
}

const (
	// ClusterMachineTemplateType is the type of the ClusterMachineTemplate resource.
	// tsgen:ClusterMachineTemplateType
	ClusterMachineTemplateType = resource.Type("ClusterMachineTemplates.omni.sidero.dev")
)

// ClusterMachineTemplate describes a cluster machine config generation template
// It's similar to generate.Input struct in the Talos repo.
type ClusterMachineTemplate = typed.Resource[ClusterMachineTemplateSpec, ClusterMachineTemplateExtension]

// ClusterMachineTemplateSpec wraps specs.ClusterMachineTemplateSpec.
type ClusterMachineTemplateSpec = protobuf.ResourceSpec[specs.ClusterMachineTemplateSpec, *specs.ClusterMachineTemplateSpec]

// ClusterMachineTemplateExtension provides auxiliary methods for ClusterMachineTemplate resource.
type ClusterMachineTemplateExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (ClusterMachineTemplateExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ClusterMachineTemplateType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "InstallImage",
				JSONPath: "{.installimage}",
			},
			{
				Name:     "Kubernetes",
				JSONPath: "{.kubernetesversion}",
			},
			{
				Name:     "Disk",
				JSONPath: "{.installdisk}",
			},
		},
	}
}
