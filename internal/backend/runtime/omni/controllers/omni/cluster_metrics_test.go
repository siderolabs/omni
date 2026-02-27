// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"maps"
	"reflect"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
)

// TestAllClusterFeaturesReported ensures that all features defined in ClusterSpec_Features
// are reported in the metrics. By this we make sure that when we introduce a new feature flag,
// we don't forget to report it in the metrics.
func TestAllClusterFeaturesReported(t *testing.T) {
	controller := omnictrl.ClusterMetricsController{}

	features := getFeatures()
	featureMetrics := controller.FeatureMetrics(slices.Values([]*omni.Cluster(nil)))

	require.Equalf(t, len(features), len(featureMetrics), "cluster features and reported feature metrics mismatch, features: %q, metric names: %q",
		features, slices.Collect(maps.Keys(featureMetrics)))
}

func getFeatures() []string {
	t := reflect.TypeFor[specs.ClusterSpec_Features]()

	var names []string

	for field := range t.Fields() {
		if field.IsExported() {
			names = append(names, field.Name)
		}
	}

	return names
}
