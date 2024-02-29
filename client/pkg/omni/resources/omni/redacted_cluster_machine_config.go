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

// NewRedactedClusterMachineConfig creates new redacted cluster machine config resource.
func NewRedactedClusterMachineConfig(ns string, id resource.ID) *RedactedClusterMachineConfig {
	return typed.NewResource[RedactedClusterMachineConfigSpec, RedactedClusterMachineConfigExtension](
		resource.NewMetadata(ns, RedactedClusterMachineConfigType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.RedactedClusterMachineConfigSpec{}),
	)
}

const (
	// RedactedClusterMachineConfigType is the type of the RedactedClusterMachineConfig resource.
	// tsgen:RedactedClusterMachineConfigType
	RedactedClusterMachineConfigType = resource.Type("RedactedClusterMachineConfigs.omni.sidero.dev")
)

// RedactedClusterMachineConfig is the redacted version of the final machine config.
type RedactedClusterMachineConfig = typed.Resource[RedactedClusterMachineConfigSpec, RedactedClusterMachineConfigExtension]

// RedactedClusterMachineConfigSpec wraps specs.RedactedClusterMachineConfigSpec.
type RedactedClusterMachineConfigSpec = protobuf.ResourceSpec[specs.RedactedClusterMachineConfigSpec, *specs.RedactedClusterMachineConfigSpec]

// RedactedClusterMachineConfigExtension provides auxiliary methods for RedactedClusterMachineConfig resource.
type RedactedClusterMachineConfigExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (RedactedClusterMachineConfigExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             RedactedClusterMachineConfigType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}
