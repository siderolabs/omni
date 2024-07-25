// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"sync"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// ClusterStatusMetricsController provides metrics based on ClusterStatus.
//
//nolint:govet
type ClusterStatusMetricsController struct {
	metricsOnce       sync.Once
	metricNumClusters *prometheus.GaugeVec
}

// Name implements controller.Controller interface.
func (ctrl *ClusterStatusMetricsController) Name() string {
	return "ClusterStatusMetricsController"
}

// Inputs implements controller.Controller interface.
func (ctrl *ClusterStatusMetricsController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: resources.DefaultNamespace,
			Type:      omni.ClusterStatusType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *ClusterStatusMetricsController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: omni.ClusterStatusMetricsType,
			Kind: controller.OutputExclusive,
		},
	}
}

func (ctrl *ClusterStatusMetricsController) initMetrics() {
	ctrl.metricsOnce.Do(func() {
		ctrl.metricNumClusters = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "omni_clusters",
			Help: "Number of cluster by status in the instance.",
		},
			[]string{"phase"})
	})
}

// Run implements controller.Controller interface.
func (ctrl *ClusterStatusMetricsController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	ctrl.initMetrics()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		list, err := safe.ReaderListAll[*omni.ClusterStatus](ctx, r)
		if err != nil {
			return err
		}

		res := omni.NewClusterStatusMetrics(resources.EphemeralNamespace, omni.ClusterStatusMetricsID)

		res.TypedSpec().Value.Phases = make(map[int32]uint32, len(specs.ClusterStatusSpec_Phase_name))

		// it is important to enumerate here all status values, as otherwise metric might not be cleared properly
		// when there are no cluster in a given status
		clustersByStatus := map[specs.ClusterStatusSpec_Phase]int{
			specs.ClusterStatusSpec_UNKNOWN:      0,
			specs.ClusterStatusSpec_SCALING_UP:   0,
			specs.ClusterStatusSpec_SCALING_DOWN: 0,
			specs.ClusterStatusSpec_RUNNING:      0,
			specs.ClusterStatusSpec_DESTROYING:   0,
		}

		for iter := list.Iterator(); iter.Next(); {
			phase := iter.Value().TypedSpec().Value.Phase
			clustersByStatus[phase]++

			res.TypedSpec().Value.Phases[int32(phase)]++

			if !iter.Value().TypedSpec().Value.Ready {
				res.TypedSpec().Value.NotReadyCount++
			}
		}

		for status, num := range clustersByStatus {
			ctrl.metricNumClusters.WithLabelValues(status.String()).Set(float64(num))
		}

		if err := safe.WriterModify(ctx, r, res, func(r *omni.ClusterStatusMetrics) error {
			r.TypedSpec().Value = res.TypedSpec().Value

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

// Describe implements prom.Collector interface.
func (ctrl *ClusterStatusMetricsController) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(ctrl, ch)
}

// Collect implements prom.Collector interface.
func (ctrl *ClusterStatusMetricsController) Collect(ch chan<- prometheus.Metric) {
	ctrl.initMetrics()

	ctrl.metricNumClusters.Collect(ch)
}

var _ prometheus.Collector = &ClusterStatusMetricsController{}
