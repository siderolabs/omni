// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package kubernetes //nolint:testpackage // exercises unexported manifestGroupForObject.

import (
	"testing"

	fluxobject "github.com/fluxcd/cli-utils/pkg/object"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestManifestGroupForClusterScopedObjectWithNamespace(t *testing.T) {
	manifest := &unstructured.Unstructured{}
	manifest.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "example.dev",
		Version: "v1",
		Kind:    "Widget",
	})
	manifest.SetNamespace("test")
	manifest.SetName("example")

	const manifestID = "Widget/test/example"

	groups := map[string]string{manifestID: "test-group"}

	id, group, ok := manifestGroupForObject(fluxobject.ObjMetadata{
		GroupKind: manifest.GroupVersionKind().GroupKind(),
		Name:      manifest.GetName(),
	}, groups, []*unstructured.Unstructured{manifest})

	require.True(t, ok)
	assert.Equal(t, manifestID, id)
	assert.Equal(t, "test-group", group)
}

func TestManifestGroupForObjectDoesNotIgnoreNamespaceForNamespacedObject(t *testing.T) {
	manifest := &unstructured.Unstructured{}
	manifest.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "example.dev",
		Version: "v1",
		Kind:    "Widget",
	})
	manifest.SetNamespace("first")
	manifest.SetName("example")

	_, _, ok := manifestGroupForObject(fluxobject.ObjMetadata{
		GroupKind: manifest.GroupVersionKind().GroupKind(),
		Namespace: "second",
		Name:      manifest.GetName(),
	}, map[string]string{"Widget/first/example": "test-group"}, []*unstructured.Unstructured{manifest})

	assert.False(t, ok)
}

func TestManifestGroupForObjectMatchesGroupKind(t *testing.T) {
	manifest := &unstructured.Unstructured{}
	manifest.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "first.example.dev",
		Version: "v1",
		Kind:    "Widget",
	})
	manifest.SetNamespace("test")
	manifest.SetName("example")

	_, _, ok := manifestGroupForObject(fluxobject.ObjMetadata{
		GroupKind: schema.GroupKind{
			Group: "second.example.dev",
			Kind:  manifest.GetKind(),
		},
		Name: manifest.GetName(),
	}, map[string]string{"Widget/test/example": "test-group"}, []*unstructured.Unstructured{manifest})

	assert.False(t, ok)
}
