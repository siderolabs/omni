// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package omni

import (
	"fmt"
	"strings"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"
	"github.com/hashicorp/go-multierror"
	"github.com/siderolabs/gen/pair"
	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	talosrole "github.com/siderolabs/talos/pkg/machinery/role"
	"gopkg.in/yaml.v3"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
)

var forbiddenFields = []string{
	"cluster.clusterName",
	"cluster.id",
	"cluster.controlPlane.endpoint",
	"cluster.token",
	"cluster.secret",
	"cluster.aescbcEncryptionSecret",
	"cluster.secretboxEncryptionSecret",
	"cluster.acceptedCAs",
	"machine.token",
	"machine.ca",
	"machine.type",
	"machine.install.extensions",
	"machine.acceptedCAs",
}

var forbiddenSliceElements = map[string]map[any]struct{}{
	"machine.features.kubernetesTalosAPIAccess.allowedRoles": {
		string(talosrole.Admin): {},
	},
}

// NewConfigPatch creates new ConfigPatch resource.
func NewConfigPatch(ns string, id resource.ID, labels ...pair.Pair[string, string]) *ConfigPatch {
	res := typed.NewResource[ConfigPatchSpec, ConfigPatchExtension](
		resource.NewMetadata(ns, ConfigPatchType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.ConfigPatchSpec{}),
	)

	for _, label := range labels {
		res.Metadata().Labels().Set(label.F1, label.F2)
	}

	return res
}

const (
	// ConfigPatchType is the type of the ConfigPatch resource.
	// tsgen:ConfigPatchType
	ConfigPatchType = resource.Type("ConfigPatches.omni.sidero.dev")
)

// ConfigPatch describes machine config patch.
type ConfigPatch = typed.Resource[ConfigPatchSpec, ConfigPatchExtension]

// ConfigPatchSpec wraps specs.ConfigPatchSpec.
type ConfigPatchSpec = protobuf.ResourceSpec[specs.ConfigPatchSpec, *specs.ConfigPatchSpec]

// ConfigPatchExtension provides auxiliary methods for ConfigPatch resource.
type ConfigPatchExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (ConfigPatchExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ConfigPatchType,
		Aliases:          []resource.Type{},
		DefaultNamespace: resources.DefaultNamespace,
		PrintColumns:     []meta.PrintColumn{},
		Sensitivity:      meta.Sensitive,
	}
}

// ValidateConfigPatch parses the config patch data using Talos config loader,
// then validates that the config patch doesn't have fields that are controlled by omni.
func ValidateConfigPatch(data string) error {
	_, err := configloader.NewFromBytes([]byte(data))
	if err != nil {
		return err
	}

	var config map[string]any

	err = yaml.Unmarshal([]byte(data), &config)
	if err != nil {
		return err
	}

	var multiErr error

	for _, field := range forbiddenFields {
		if _, ok := getField(config, field); ok {
			multiErr = multierror.Append(multiErr, fmt.Errorf("overriding %q is not allowed in the config patch", field))
		}
	}

	for field, forbiddenElementSet := range forbiddenSliceElements {
		val, ok := getField(config, field)
		if !ok {
			continue
		}

		slice, ok := val.([]any)
		if !ok {
			continue
		}

		for _, element := range slice {
			if _, ok = forbiddenElementSet[element]; ok {
				multiErr = multierror.Append(multiErr, fmt.Errorf("element %q is not allowed in field %q", element, field))
			}
		}
	}

	return multiErr
}

func getField(config map[string]any, field string) (any, bool) {
	parts := strings.Split(field, ".")

	var obj any

	obj = config
	for _, part := range parts {
		current, ok := obj.(map[string]any)
		if !ok {
			return nil, false
		}

		obj, ok = current[part]
		if !ok {
			return nil, false
		}
	}

	return obj, true
}
