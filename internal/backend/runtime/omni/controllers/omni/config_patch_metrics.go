// Copyright (c) 2025 Sidero Labs, Inc.
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

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// ConfigPatchMetricsController publishes metrics on config patches.
//
//nolint:govet
type ConfigPatchMetricsController struct {
	metricsOnce sync.Once
	patchSizes  *prometheus.HistogramVec
}

// Name implements controller.Controller interface.
func (ctrl *ConfigPatchMetricsController) Name() string {
	return "ConfigPatchMetricsController"
}

// Inputs implements controller.Controller interface.
func (ctrl *ConfigPatchMetricsController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: resources.DefaultNamespace,
			Type:      omni.ConfigPatchType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *ConfigPatchMetricsController) Outputs() []controller.Output {
	return nil
}

func (ctrl *ConfigPatchMetricsController) initMetrics() {
	ctrl.metricsOnce.Do(func() {
		ctrl.patchSizes = prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name: "omni_config_patch_size",
			Help: "Size of the config patch",
			Buckets: []float64{
				0,
				1024,
				100 * 1024,
				512 * 1024,
				1 * 1024 * 1024,
				10 * 1024 * 1024,
				20 * 1024 * 1024,
			},
		}, []string{"state"})
	})
}

// Run implements controller.Controller interface.
func (ctrl *ConfigPatchMetricsController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	ctrl.initMetrics()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		allConfigPatches, err := safe.ReaderListAll[*omni.ConfigPatch](ctx, r)
		if err != nil {
			return err
		}

		for p := range allConfigPatches.All() {
			if data := p.TypedSpec().Value.GetData(); len(data) > 0 {
				ctrl.patchSizes.WithLabelValues("uncompressed").Observe(float64(len(data)))
			} else {
				ctrl.patchSizes.WithLabelValues("compressed").Observe(float64(len(p.TypedSpec().Value.GetCompressedData())))
			}
		}

		select {
		case <-ctx.Done():
			return nil
		case <-time.After(10 * time.Second): // don't reconcile too often, as metrics are not scraped that often
		}
	}
}

// Describe implements prom.Collector interface.
func (ctrl *ConfigPatchMetricsController) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(ctrl, ch)
}

// Collect implements prom.Collector interface.
func (ctrl *ConfigPatchMetricsController) Collect(ch chan<- prometheus.Metric) {
	ctrl.initMetrics()

	ctrl.patchSizes.Collect(ch)
}

var _ prometheus.Collector = &ConfigPatchMetricsController{}
