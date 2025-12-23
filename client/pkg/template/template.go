// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package template provides conversion of cluster templates to Omni resources.
package template

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xslices"
	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/template/internal/models"
)

// Template is a cluster template.
type Template struct {
	models models.List
}

// Load the template from input.
func Load(input io.Reader) (*Template, error) {
	dec := yaml.NewDecoder(input)

	var template Template

	for {
		var docNode yaml.Node

		if err := dec.Decode(&docNode); err != nil {
			if errors.Is(err, io.EOF) {
				return &template, nil
			}

			return nil, fmt.Errorf("error decoding template: %w", err)
		}

		if docNode.Kind != yaml.DocumentNode {
			return nil, fmt.Errorf("unexpected node kind %q", docNode.Kind)
		}

		if len(docNode.Content) != 1 {
			return nil, fmt.Errorf("unexpected number of nodes %d", len(docNode.Content))
		}

		kind, err := findKind(docNode.Content[0])
		if err != nil {
			return nil, fmt.Errorf("error in document at line %d:%d: %w", docNode.Line, docNode.Column, err)
		}

		model, err := models.New(kind)
		if err != nil {
			return nil, fmt.Errorf("error in document at line %d:%d: %w", docNode.Line, docNode.Column, err)
		}

		// YAML decoder doesn't allow to decode with KnownFields: true from a Node
		// so we do a roundtrip to bytes and back :sigh:
		raw, err := yaml.Marshal(docNode.Content[0])
		if err != nil {
			return nil, fmt.Errorf("error marshaling document at line %d:%d: %w", docNode.Line, docNode.Column, err)
		}

		documentDecoder := yaml.NewDecoder(bytes.NewReader(raw))
		documentDecoder.KnownFields(true)

		if err = documentDecoder.Decode(model); err != nil {
			return nil, fmt.Errorf("error decoding document at line %d:%d: %w", docNode.Line, docNode.Column, err)
		}

		template.models = append(template.models, model)
	}
}

func findKind(node *yaml.Node) (string, error) {
	if node.Kind != yaml.MappingNode {
		return "", fmt.Errorf("unexpected node kind %q, expecting mapping", node.Kind)
	}

	for i := 0; i < len(node.Content); i += 2 {
		key := node.Content[i]
		value := node.Content[i+1]

		if key.Kind != yaml.ScalarNode {
			return "", fmt.Errorf("unexpected node kind %q", key.Kind)
		}

		if key.Value != "kind" {
			continue
		}

		if value.Kind != yaml.ScalarNode {
			return "", fmt.Errorf("unexpected value type for kind field %q", value.Kind)
		}

		return value.Value, nil
	}

	return "", fmt.Errorf("kind field not found")
}

// WithCluster creates an empty template which contains only cluster model.
//
// Such template can be used for reading a cluster status, deleting a cluster, etc.
func WithCluster(clusterName string) *Template {
	return &Template{
		models: models.List{
			&models.Cluster{
				Meta: models.Meta{
					Kind: models.KindCluster,
				},
				Name: clusterName,
			},
		},
	}
}

// Validate the template.
func (t *Template) Validate() error {
	return t.models.Validate()
}

// Translate the template into resources.
func (t *Template) Translate() ([]resource.Resource, error) {
	return t.models.Translate()
}

// ClusterName returns the name of the cluster associated with the template.
func (t *Template) ClusterName() (string, error) {
	return t.models.ClusterName()
}

// actualResources returns a list of resources in the state related to the cluster template.
func (t *Template) actualResources(ctx context.Context, st state.State) ([]resource.Resource, error) {
	clusterName, err := t.models.ClusterName()
	if err != nil {
		return nil, err
	}

	var actualResources []resource.Resource

	clusterResource, err := st.Get(ctx, resource.NewMetadata(resources.DefaultNamespace, omni.ClusterType, clusterName, resource.VersionUndefined))
	if err != nil {
		if !state.IsNotFoundError(err) {
			return nil, err
		}
	} else {
		actualResources = append(actualResources, clusterResource)
	}

	for _, resourceType := range []resource.Type{
		omni.MachineSetType,
		omni.MachineSetNodeType,
		omni.ConfigPatchType,
		omni.ExtensionsConfigurationType,
	} {
		items, err := st.List(
			ctx,
			resource.NewMetadata(resources.DefaultNamespace, resourceType, "", resource.VersionUndefined),
			state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, clusterName)),
		)
		if err != nil {
			return nil, err
		}

		actualResources = append(actualResources,
			xslices.Filter(items.Items,
				func(r resource.Resource) bool {
					_, programmaticallyCreatedMachineSetNode := r.Metadata().Labels().Get(omni.LabelManagedByMachineSetNodeController)

					return !programmaticallyCreatedMachineSetNode && r.Metadata().Owner() == ""
				},
			)...)
	}

	return actualResources, nil
}

func splitResourcesToDelete(toDelete []resource.Resource) [][]resource.Resource {
	phases := make([][]resource.Resource, 2)

	for _, r := range deduplicateDeletion(toDelete) {
		switch r.Metadata().Type() {
		case omni.MachineSetNodeType, omni.MachineSetType:
			phases[0] = append(phases[0], r)
		default:
			phases[1] = append(phases[1], r)
		}
	}

	for i := range phases {
		sortResources(phases[i], func(r resource.Resource) resource.Metadata { return *r.Metadata() })
	}

	return phases
}

// Delete returns a sync result which lists what needs to be deleted from state to remove template from the cluster.
func (t *Template) Delete(ctx context.Context, st state.State) (*SyncResult, error) {
	actualResources, err := t.actualResources(ctx, st)
	if err != nil {
		return nil, err
	}

	syncResult := SyncResult{
		Destroy: splitResourcesToDelete(actualResources),
	}

	return &syncResult, nil
}

func metadataKey(md resource.Metadata) string {
	return fmt.Sprintf("%s/%s/%s", md.Namespace(), md.Type(), md.ID())
}

// Sync the template against the resource state.
func (t *Template) Sync(ctx context.Context, st state.State) (*SyncResult, error) {
	clusterName, err := t.models.ClusterName()
	if err != nil {
		return nil, err
	}

	expectedResources, err := t.Translate()
	if err != nil {
		return nil, err
	}

	actualResources, err := t.actualResources(ctx, st)
	if err != nil {
		return nil, err
	}

	expectedResourceMap := xslices.ToMap(expectedResources, func(r resource.Resource) (string, resource.Resource) {
		return metadataKey(*r.Metadata()), r
	})

	var (
		syncResult SyncResult
		toDelete   []resource.Resource
	)

	for _, actualResource := range actualResources {
		if expectedResource, ok := expectedResourceMap[metadataKey(*actualResource.Metadata())]; ok {
			// copy some metadata to minimize the diff
			expectedResource.Metadata().SetVersion(actualResource.Metadata().Version())
			expectedResource.Metadata().SetUpdated(actualResource.Metadata().Updated())
			expectedResource.Metadata().SetCreated(actualResource.Metadata().Created())
			expectedResource.Metadata().Finalizers().Set(*actualResource.Metadata().Finalizers())

			if !resource.Equal(actualResource, expectedResource) {
				syncResult.Update = append(syncResult.Update, UpdateChange{Old: actualResource, New: expectedResource})
			}
		} else {
			// check that actual resource belongs to the cluster to avoid removing resources from other clusters
			if actualResource.Metadata().Type() != omni.ClusterType {
				if clusterLabel, _ := actualResource.Metadata().Labels().Get(omni.LabelCluster); clusterLabel != clusterName {
					return nil, fmt.Errorf("resource %s belongs to cluster %q, but template is for cluster %q", resource.String(actualResource), clusterLabel, clusterName)
				}
			}

			toDelete = append(toDelete, actualResource)
		}

		delete(expectedResourceMap, metadataKey(*actualResource.Metadata()))
	}

	for _, expectedResource := range expectedResourceMap {
		if _, ok := expectedResourceMap[metadataKey(*expectedResource.Metadata())]; ok {
			// check that no such resource exists (for a different cluster)
			if actualResource, err := st.Get(ctx, *expectedResource.Metadata()); err == nil {
				clusterLabel, _ := actualResource.Metadata().Labels().Get(omni.LabelCluster)

				return nil, fmt.Errorf("resource %s already exists from cluster %q, but template is for cluster %q", resource.String(actualResource), clusterLabel, clusterName)
			}

			syncResult.Create = append(syncResult.Create, expectedResource)
		}
	}

	sortResources(syncResult.Create, func(r resource.Resource) resource.Metadata { return *r.Metadata() })
	sortResources(syncResult.Update, func(u UpdateChange) resource.Metadata { return *u.New.Metadata() })
	syncResult.Destroy = splitResourcesToDelete(toDelete)

	return &syncResult, nil
}

func deduplicateDeletion(toDelete []resource.Resource) []resource.Resource {
	toDeleteMap := xslices.ToMap(toDelete, func(r resource.Resource) (string, resource.Resource) {
		return metadataKey(*r.Metadata()), r
	})

	r := xslices.Filter(toDelete, func(r resource.Resource) bool {
		switch r.Metadata().Type() {
		case omni.ClusterType:
			return true
		case omni.MachineSetNodeType:
			machineSetName, ok := r.Metadata().Labels().Get(omni.LabelMachineSet)
			if !ok {
				return true
			}

			if _, ok := toDeleteMap[metadataKey(resource.NewMetadata(resources.DefaultNamespace, omni.MachineSetType, machineSetName, resource.VersionUndefined))]; ok {
				return false
			}
		default:
			clusterName, ok := r.Metadata().Labels().Get(omni.LabelCluster)
			if !ok {
				return true
			}

			if _, ok := toDeleteMap[metadataKey(resource.NewMetadata(resources.DefaultNamespace, omni.ClusterType, clusterName, resource.VersionUndefined))]; ok {
				return false
			}
		}

		return true
	})

	return r
}
