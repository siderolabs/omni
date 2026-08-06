// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package omni

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"
	"github.com/hashicorp/go-multierror"
	"github.com/siderolabs/gen/pair"
	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	taloscluster "github.com/siderolabs/talos/pkg/machinery/config/types/cluster"
	talosk8s "github.com/siderolabs/talos/pkg/machinery/config/types/k8s"
	talosruntime "github.com/siderolabs/talos/pkg/machinery/config/types/runtime"
	talosrole "github.com/siderolabs/talos/pkg/machinery/role"
	"go.yaml.in/yaml/v4"

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
	"cluster.ca",
	"cluster.acceptedCAs",
	"cluster.serviceAccount",
	"machine.token",
	"machine.ca",
	"machine.type",
	"machine.install.disk",
	"machine.install.extensions",
	"machine.acceptedCAs",
}

var forbiddenDocuments = map[string]struct{}{
	talosruntime.UnattendedInstallConfigKind: {}, // not supported in Omni
	taloscluster.DiscoveryIdentityKind:       {}, // cluster.id, cluster.secret
	talosk8s.KubeClusterConfig:               {}, // cluster.clusterName, cluster.controlPlane.endpoint
	talosk8s.KubeEtcdEncryptionConfig:        {}, // cluster.secretboxEncryptionSecret, cluster.aescbcEncryptionSecret
	talosk8s.KubeAPIServerCAConfig:           {}, // the Kubernetes CA, private key included
	talosk8s.KubeAggregatorCAConfig:          {}, // the aggregator CA, private key included
}

var documentForbiddenFields = map[string][]string{
	talosk8s.KubeServiceAccountConfig: {"issuer.privateKey"},
}

type forbiddenElements = map[string]map[any]struct{}

var forbiddenSliceElements = forbiddenElements{
	"machine.features.kubernetesTalosAPIAccess.allowedRoles": {
		string(talosrole.Admin): {},
	},
}

var documentForbiddenSliceElements = map[string]forbiddenElements{
	talosk8s.KubeTalosAPIAccessConfig: {
		"allowedRoles": {
			string(talosrole.Admin): {},
		},
	},
}

// NewConfigPatch creates new ConfigPatch resource.
func NewConfigPatch(id resource.ID, labels ...pair.Pair[string, string]) *ConfigPatch {
	res := typed.NewResource[ConfigPatchSpec, ConfigPatchExtension](
		resource.NewMetadata(resources.DefaultNamespace, ConfigPatchType, id, resource.VersionUndefined),
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
func ValidateConfigPatch(data []byte) error {
	_, err := configloader.NewFromBytes(data, configloader.WithAllowPatchDelete())
	if err != nil {
		return err
	}

	documents, err := decodeFromYAML(data)
	if err != nil {
		return err
	}

	var multiErr error

	// every document has to be checked: the v1alpha1 one that forbiddenFields describes can appear
	// at any position, and a forbidden document can appear more than once
	for _, document := range documents {
		if kind, ok := documentKind(document); ok {
			if _, forbidden := forbiddenDocuments[kind]; forbidden {
				multiErr = multierror.Append(multiErr, fmt.Errorf("the %q document is not allowed in the config patch", kind))

				continue
			}
		}

		if documentErr := validateConfigPatchDocument(document); documentErr != nil {
			multiErr = multierror.Append(multiErr, documentErr)
		}
	}

	return multiErr
}

func validateConfigPatchDocument(document map[string]any) error {
	var multiErr error

	for _, field := range documentFields(document) {
		if _, ok := getField(document, field); ok {
			multiErr = multierror.Append(multiErr, fmt.Errorf("overriding %q is not allowed in the config patch", field))
		}
	}

	for field, forbiddenElementSet := range documentElements(document) {
		val, ok := getField(document, field)
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

// documentKind returns the kind of a multi-document config document. The legacy v1alpha1 document
// carries no kind, so it reports false.
func documentKind(document map[string]any) (string, bool) {
	kind, ok := document["kind"].(string)

	return kind, ok
}

// documentFields returns the field restrictions that apply to the document.
func documentFields(document map[string]any) []string {
	kind, ok := documentKind(document)
	if !ok {
		return forbiddenFields
	}

	return documentForbiddenFields[kind]
}

// documentElements returns the element restrictions that apply to the document.
func documentElements(document map[string]any) forbiddenElements {
	kind, ok := documentKind(document)
	if !ok {
		return forbiddenSliceElements
	}

	return documentForbiddenSliceElements[kind]
}

// SanitizeConfigPatch parses the config patch data using Talos config loader,
// then sanitizes that the config patch so it doesn't have fields that are controlled by omni.
func SanitizeConfigPatch(data []byte) ([]byte, error) {
	_, err := configloader.NewFromBytes(data, configloader.WithAllowPatchDelete())
	if err != nil {
		return nil, err
	}

	patches, err := decodeFromYAML(data)
	if err != nil {
		return nil, err
	}

	sanitizedPatches := make([]map[string]any, 0, len(patches))

	for _, patch := range patches {
		// filter out patches for forbidden documents altogether
		if kind, ok := documentKind(patch); ok {
			if _, forbidden := forbiddenDocuments[kind]; forbidden {
				continue
			}
		}

		sanitized := sanitizeConfigPatch(patch)

		// a patch that held nothing but Omni-controlled fields is dropped rather than emitted as an
		// empty document, or as a kind header that patches nothing
		if isEmptyDocument(sanitized) {
			continue
		}

		sanitizedPatches = append(sanitizedPatches, sanitized)
	}

	return encodeToYAML(sanitizedPatches)
}

func sanitizeConfigPatch(config map[string]any) map[string]any {
	for _, field := range documentFields(config) {
		if _, ok := getField(config, field); ok {
			removeField(config, field)
		}
	}

	for key, val := range config {
		nestedMap, ok := val.(map[string]any)
		if !ok {
			continue
		}

		if len(nestedMap) == 0 {
			delete(config, key)
		}
	}

	for field, forbiddenElementSet := range documentElements(config) {
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
				removeFieldElement(config, field, element)
			}
		}
	}

	return config
}

// isEmptyDocument reports whether the document carries nothing beyond its meta header.
func isEmptyDocument(document map[string]any) bool {
	for key := range document {
		if key != "apiVersion" && key != "kind" {
			return false
		}
	}

	return true
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

func removeField(m map[string]any, field string) {
	parts := strings.Split(field, ".")

	for i, part := range parts {
		if i == len(parts)-1 {
			// Last part — delete the key if it exists
			delete(m, part)

			return
		}

		next, ok := m[part]
		if !ok {
			return
		}

		nextMap, ok := next.(map[string]any)
		if !ok {
			return
		}

		m = nextMap
	}
}

func removeFieldElement(m map[string]any, field string, element any) {
	parts := strings.Split(field, ".")

	for i, part := range parts {
		if i == len(parts)-1 {
			// Last part — delete the element if it exists
			val, ok := m[part]
			if !ok {
				return
			}

			slice, ok := val.([]any)
			if !ok {
				return
			}

			sliceCopy := make([]any, 0, len(slice))
			for _, elem := range slice {
				// Populate the slice without the forbidden element
				if elem != element {
					sliceCopy = append(sliceCopy, elem)
				}
			}

			if len(slice) == len(sliceCopy) {
				return
			}

			if len(sliceCopy) == 0 {
				// If the slice is now empty, remove the key entirely
				delete(m, part)

				return
			}
			// Update the field with the modified slice
			m[part] = sliceCopy

			return
		}

		next, ok := m[part]
		if !ok {
			return
		}

		nextMap, ok := next.(map[string]any)
		if !ok {
			return
		}

		m = nextMap
	}
}

func encodeToYAML(docs []map[string]any) ([]byte, error) {
	// the encoder rejects a Close without a document, which happens when every document was dropped
	if len(docs) == 0 {
		return nil, nil
	}

	var buf bytes.Buffer

	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)

	for _, doc := range docs {
		if err := enc.Encode(doc); err != nil {
			return nil, err
		}
	}

	if err := enc.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func decodeFromYAML(data []byte) ([]map[string]any, error) {
	input := bytes.NewReader(data)
	dec := yaml.NewDecoder(input)

	documents := make([]map[string]any, 0)

	for {
		var document map[string]any
		if err := dec.Decode(&document); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return nil, err
		}

		documents = append(documents, document)
	}

	return documents, nil
}
