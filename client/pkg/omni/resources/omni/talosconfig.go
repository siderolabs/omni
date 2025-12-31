// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package omni

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"
	clientconfig "github.com/siderolabs/talos/pkg/machinery/client/config"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
)

// NewTalosConfig creates new Talos config resource.
func NewTalosConfig(id resource.ID) *TalosConfig {
	return typed.NewResource[TalosConfigSpec, TalosConfigExtension](
		resource.NewMetadata(resources.DefaultNamespace, TalosConfigType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.TalosConfigSpec{}),
	)
}

const (
	// TalosConfigType is the type of the TalosConfig resource.
	TalosConfigType = resource.Type("TalosConfigs.omni.sidero.dev")
)

// TalosConfig describes client config for Talos API.
type TalosConfig = typed.Resource[TalosConfigSpec, TalosConfigExtension]

// TalosConfigSpec wraps specs.TalosConfigSpec.
type TalosConfigSpec = protobuf.ResourceSpec[specs.TalosConfigSpec, *specs.TalosConfigSpec]

// TalosConfigExtension provides auxiliary methods for TalosConfig resource.
type TalosConfigExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (TalosConfigExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             TalosConfigType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns:     []meta.PrintColumn{},
	}
}

// NewTalosClientConfig creates talos config.Config from TalosConfig resource.
func NewTalosClientConfig(in *TalosConfig, endpoints ...string) *clientconfig.Config {
	spec := in.TypedSpec().Value

	config := &clientconfig.Config{
		Context: in.Metadata().ID(),
		Contexts: map[string]*clientconfig.Context{
			in.Metadata().ID(): {
				Endpoints: endpoints,
				CA:        spec.Ca,
				Crt:       spec.Crt,
				Key:       spec.Key,
			},
		},
	}

	return config
}
