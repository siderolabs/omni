// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"iter"
	"sync"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// ClusterMetricsController provides metrics based on Cluster.
//
//nolint:govet
type ClusterMetricsController struct {
	metricsOnce              sync.Once
	metricNumClusterFeatures *prometheus.GaugeVec
}

// Name implements controller.Controller interface.
func (ctrl *ClusterMetricsController) Name() string {
	return "ClusterMetricsController"
}

// Inputs implements controller.Controller interface.
func (ctrl *ClusterMetricsController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: resources.DefaultNamespace,
			Type:      omni.ClusterType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *ClusterMetricsController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: omni.ClusterMetricsType,
			Kind: controller.OutputExclusive,
		},
	}
}

func (ctrl *ClusterMetricsController) initMetrics() {
	ctrl.metricsOnce.Do(func() {
		ctrl.metricNumClusterFeatures = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "omni_cluster_features",
			Help: "Number of clusters with specific features enabled.",
		}, []string{"feature"})
	})
}

// Run implements controller.Controller interface.
func (ctrl *ClusterMetricsController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	ctrl.initMetrics()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		clusterList, err := safe.ReaderListAll[*omni.Cluster](ctx, r)
		if err != nil {
			return err
		}

		featureMetrics := ctrl.FeatureMetrics(clusterList.All())

		for feature, num := range featureMetrics {
			ctrl.metricNumClusterFeatures.WithLabelValues(feature).Set(float64(num))
		}

		if err = safe.WriterModify(ctx, r, omni.NewClusterMetrics(omni.ClusterMetricsID), func(res *omni.ClusterMetrics) error {
			res.TypedSpec().Value.Features = featureMetrics

			return nil
		}); err != nil {
			return err
		}

		select {
		case <-ctx.Done():
			return nil
		case <-time.After(10 * time.Second): // don't reconcile too often, as metrics are not scraped that often
		}
	}
}

func (ctrl *ClusterMetricsController) FeatureMetrics(clusters iter.Seq[*omni.Cluster]) map[string]uint32 {
	const (
		featureWorkloadProxy            = "workload_proxy"
		featureDiskEncryption           = "disk_encryption"
		featureEmbeddedDiscoveryService = "embedded_discovery_service"
	)

	featuresByCluster := map[string]uint32{
		featureWorkloadProxy:            0,
		featureDiskEncryption:           0,
		featureEmbeddedDiscoveryService: 0,
	}

	for cluster := range clusters {
		if cluster.TypedSpec().Value.GetFeatures().GetEnableWorkloadProxy() {
			featuresByCluster[featureWorkloadProxy]++
		}

		if cluster.TypedSpec().Value.GetFeatures().GetDiskEncryption() {
			featuresByCluster[featureDiskEncryption]++
		}

		if cluster.TypedSpec().Value.GetFeatures().GetUseEmbeddedDiscoveryService() {
			featuresByCluster[featureEmbeddedDiscoveryService]++
		}
	}

	return featuresByCluster
}

// Describe implements prom.Collector interface.
func (ctrl *ClusterMetricsController) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(ctrl, ch)
}

// Collect implements prom.Collector interface.
func (ctrl *ClusterMetricsController) Collect(ch chan<- prometheus.Metric) {
	ctrl.initMetrics()

	ctrl.metricNumClusterFeatures.Collect(ch)
}

var _ prometheus.Collector = &ClusterMetricsController{}
