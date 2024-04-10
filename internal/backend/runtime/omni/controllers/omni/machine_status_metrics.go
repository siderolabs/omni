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

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// MachineStatusMetricsController provides metrics based on ClusterStatus.
//
//nolint:govet
type MachineStatusMetricsController struct {
	versionsMu  sync.Mutex
	versionsMap map[string]int

	metricsOnce                 sync.Once
	metricNumMachines           prometheus.Gauge
	metricNumConnectedMachines  prometheus.Gauge
	metricNumMachinesPerVersion *prometheus.Desc
}

// Name implements controller.Controller interface.
func (ctrl *MachineStatusMetricsController) Name() string {
	return "MachineStatusMetricsController"
}

// Inputs implements controller.Controller interface.
func (ctrl *MachineStatusMetricsController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: resources.DefaultNamespace,
			Type:      omni.MachineStatusType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *MachineStatusMetricsController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: omni.MachineStatusMetricsType,
			Kind: controller.OutputExclusive,
		},
	}
}

func (ctrl *MachineStatusMetricsController) initMetrics() {
	ctrl.metricsOnce.Do(func() {
		ctrl.metricNumMachines = prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "omni_machines",
			Help: "Number of machines in the instance.",
		})

		ctrl.metricNumConnectedMachines = prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "omni_connected_machines",
			Help: "Number of machines in the instance that are connected.",
		})

		ctrl.metricNumMachinesPerVersion = prometheus.NewDesc(
			"omni_machines_version",
			"Number of machines in the instance by version.",
			[]string{"talos_version"},
			nil,
		)
	})
}

// Run implements controller.Controller interface.
func (ctrl *MachineStatusMetricsController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	ctrl.initMetrics()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		list, err := safe.ReaderListAll[*omni.MachineStatus](
			ctx,
			r,
		)
		if err != nil {
			return err
		}

		var machines, connectedMachines, allocatedMachines int

		ctrl.versionsMu.Lock()
		ctrl.versionsMap = map[string]int{}

		for iter := list.Iterator(); iter.Next(); {
			machines++

			if iter.Value().TypedSpec().Value.Connected {
				connectedMachines++
			}

			if iter.Value().TypedSpec().Value.TalosVersion != "" {
				ctrl.versionsMap[iter.Value().TypedSpec().Value.TalosVersion]++
			}

			if iter.Value().TypedSpec().Value.Cluster != "" {
				allocatedMachines++
			}
		}

		ctrl.versionsMu.Unlock()

		ctrl.metricNumMachines.Set(float64(machines))
		ctrl.metricNumConnectedMachines.Set(float64(connectedMachines))

		if err = safe.WriterModify(ctx, r, omni.NewMachineStatusMetrics(resources.EphemeralNamespace, omni.MachineStatusMetricsID),
			func(res *omni.MachineStatusMetrics) error {
				res.TypedSpec().Value.ConnectedMachinesCount = uint32(connectedMachines)
				res.TypedSpec().Value.RegisteredMachinesCount = uint32(machines)
				res.TypedSpec().Value.AllocatedMachinesCount = uint32(allocatedMachines)

				return nil
			},
		); err != nil {
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
func (ctrl *MachineStatusMetricsController) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(ctrl, ch)
}

// Collect implements prom.Collector interface.
func (ctrl *MachineStatusMetricsController) Collect(ch chan<- prometheus.Metric) {
	ctrl.initMetrics()

	ctrl.versionsMu.Lock()

	for version, count := range ctrl.versionsMap {
		ch <- prometheus.MustNewConstMetric(ctrl.metricNumMachinesPerVersion, prometheus.GaugeValue, float64(count), version)
	}

	ctrl.versionsMu.Unlock()

	ctrl.metricNumMachines.Collect(ch)
	ctrl.metricNumConnectedMachines.Collect(ch)
}

var _ prometheus.Collector = &MachineStatusMetricsController{}
