// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package kubernetes //nolint:testpackage // exercises unexported manifestGroupForObject.

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	fluxobject "github.com/fluxcd/cli-utils/pkg/object"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/siderolabs/omni/client/api/omni/specs"
)

func TestManifestApplyWaitTimeout(t *testing.T) {
	assert.Equal(t, 60*time.Second, manifestApplyWaitTimeout)
	assert.Greater(t, manifestApplyWaitTimeout, 5*time.Second)
}

func TestRetryableManifestLastError(t *testing.T) {
	assert.Equal(t, "retryable error occurred while applying manifests: context deadline exceeded", retryableManifestLastError(context.DeadlineExceeded))

	message := retryableManifestLastError(errors.Join(context.DeadlineExceeded, errors.New(strings.Repeat("x", 512))))
	assert.NotContains(t, message, "\n")
	assert.LessOrEqual(t, len(message), 512)
	assert.True(t, strings.HasSuffix(message, "..."))

	// Continuation bytes and a wide join stay inside the cap and stay one line.
	huge := retryableManifestLastError(errors.New(strings.Repeat("\x80", 4096)))
	assert.LessOrEqual(t, len(huge), 512)
	assert.NotContains(t, huge, "\n")
	assert.True(t, strings.HasPrefix(huge, "retryable error occurred while applying manifests"))
	assert.True(t, strings.HasSuffix(huge, "..."))

	parts := make([]error, 1000)
	for i := range parts {
		parts[i] = errors.New("boom")
	}

	wide := retryableManifestLastError(errors.Join(parts...))
	assert.LessOrEqual(t, len(wide), 512)
	assert.NotContains(t, wide, "\n")
	assert.True(t, strings.HasPrefix(wide, "retryable error occurred while applying manifests"))
	assert.Contains(t, wide, "boom")
}

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

func TestIsLegacyGroupID(t *testing.T) {
	for _, tt := range []struct {
		name         string
		oldID, newID string
		want         bool
	}{
		{"weighted match", "200-cluster-edge-migration", "cluster-edge-migration", true},
		{"identical strings are not legacy", "cluster-edge-migration", "cluster-edge-migration", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isLegacyGroupID(tt.oldID, tt.newID))
		})
	}
}

func TestMatchLegacyAppliedGroups(t *testing.T) {
	groups := map[string]*specs.ClusterKubernetesManifestsStatusSpec_GroupStatus{
		"200-cluster-x-a": {
			Mode:  specs.KubernetesManifestGroupSpec_ONE_TIME,
			Phase: specs.ClusterKubernetesManifestsStatusSpec_GroupStatus_APPLIED,
		},
		"201-cluster-x-b": {
			Mode:  specs.KubernetesManifestGroupSpec_ONE_TIME,
			Phase: specs.ClusterKubernetesManifestsStatusSpec_GroupStatus_PROGRESSING,
		},
		"cluster-x-c": {
			Mode:  specs.KubernetesManifestGroupSpec_FULL,
			Phase: specs.ClusterKubernetesManifestsStatusSpec_GroupStatus_APPLIED,
		},
	}

	assert.Equal(t, []string{"200-cluster-x-a"}, matchLegacyAppliedGroups(groups, "cluster-x-a"))
	assert.Empty(t, matchLegacyAppliedGroups(groups, "cluster-x-b")) // not APPLIED, excluded
	assert.Empty(t, matchLegacyAppliedGroups(groups, "cluster-x-c")) // FULL mode, excluded even though same "shape"
}
