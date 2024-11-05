// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package infra

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
)

// NewConfigPatchRequest creates new ConfigPatchRequest resource.
func NewConfigPatchRequest(ns string, id resource.ID) *ConfigPatchRequest {
	return typed.NewResource[ConfigPatchRequestSpec, ConfigPatchRequestExtension](
		resource.NewMetadata(ns, ConfigPatchRequestType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.ConfigPatchSpec{}),
	)
}

const (
	// ConfigPatchRequestType is the type of the ConfigPatch resource.
	// tsgen:ConfigPatchRequestType
	ConfigPatchRequestType = resource.Type("ConfigPatchRequests.omni.sidero.dev")
)

// ConfigPatchRequest requests a config patch to be created for the machine.
// The controller should copy this resource contents to the target config patch, if the patch is valid.
type ConfigPatchRequest = typed.Resource[ConfigPatchRequestSpec, ConfigPatchRequestExtension]

// ConfigPatchRequestSpec wraps specs.ConfigPatchRequestSpec.
type ConfigPatchRequestSpec = protobuf.ResourceSpec[specs.ConfigPatchSpec, *specs.ConfigPatchSpec]

// ConfigPatchRequestExtension provides auxiliary methods for ConfigPatch resource.
type ConfigPatchRequestExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (ConfigPatchRequestExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ConfigPatchRequestType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.InfraProviderNamespace,
		PrintColumns:     []meta.PrintColumn{},
		Sensitivity:      meta.Sensitive,
	}
}
