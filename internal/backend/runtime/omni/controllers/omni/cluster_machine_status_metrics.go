// Copyright (c) 2026 Sidero Labs, Inc.
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

// ClusterMachineStatusMetricsController publishes metrics on cluster machines.
//
//nolint:govet
type ClusterMachineStatusMetricsController struct {
	metricsOnce       sync.Once
	metricNumMachines *prometheus.GaugeVec
}

// Name implements controller.Controller interface.
func (ctrl *ClusterMachineStatusMetricsController) Name() string {
	return "ClusterMachineStatusMetricsController"
}

// Inputs implements controller.Controller interface.
func (ctrl *ClusterMachineStatusMetricsController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: resources.DefaultNamespace,
			Type:      omni.ClusterMachineStatusType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *ClusterMachineStatusMetricsController) Outputs() []controller.Output {
	return nil
}

func (ctrl *ClusterMachineStatusMetricsController) initMetrics() {
	ctrl.metricsOnce.Do(func() {
		ctrl.metricNumMachines = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "omni_cluster_machines",
			Help: "Number of cluster machines by status in the instance.",
		},
			[]string{"stage"})
	})
}

// Run implements controller.Controller interface.
func (ctrl *ClusterMachineStatusMetricsController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	ctrl.initMetrics()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		clusterMachineStatusList, err := safe.ReaderListAll[*omni.ClusterMachineStatus](ctx, r)
		if err != nil {
			return err
		}

		// it is important to enumerate here all status values, as otherwise metric might not be cleared properly
		// when there are no cluster in a given status
		clusterMachinesByStatus := map[specs.ClusterMachineStatusSpec_Stage]int{
			specs.ClusterMachineStatusSpec_UNKNOWN:       0,
			specs.ClusterMachineStatusSpec_BOOTING:       0,
			specs.ClusterMachineStatusSpec_INSTALLING:    0,
			specs.ClusterMachineStatusSpec_UPGRADING:     0,
			specs.ClusterMachineStatusSpec_CONFIGURING:   0,
			specs.ClusterMachineStatusSpec_RUNNING:       0,
			specs.ClusterMachineStatusSpec_REBOOTING:     0,
			specs.ClusterMachineStatusSpec_SHUTTING_DOWN: 0,
			specs.ClusterMachineStatusSpec_DESTROYING:    0,
		}

		for val := range clusterMachineStatusList.All() {
			clusterMachinesByStatus[val.TypedSpec().Value.Stage]++
		}

		for status, num := range clusterMachinesByStatus {
			ctrl.metricNumMachines.WithLabelValues(status.String()).Set(float64(num))
		}

		select {
		case <-ctx.Done():
			return nil
		case <-time.After(10 * time.Second): // don't reconcile too often, as metrics are not scraped that often
		}
	}
}

// Describe implements prom.Collector interface.
func (ctrl *ClusterMachineStatusMetricsController) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(ctrl, ch)
}

// Collect implements prom.Collector interface.
func (ctrl *ClusterMachineStatusMetricsController) Collect(ch chan<- prometheus.Metric) {
	ctrl.initMetrics()

	ctrl.metricNumMachines.Collect(ch)
}

var _ prometheus.Collector = &ClusterMachineStatusMetricsController{}
