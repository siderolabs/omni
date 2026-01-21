// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"iter"
	"strconv"
	"sync"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/siderolabs/talos/pkg/machinery/platforms"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

type nodeInfo struct {
	talosVersion string
	cluster      string
	connected    bool
}

// MachineStatusMetricsController provides metrics based on ClusterStatus.
//
//nolint:govet
type MachineStatusMetricsController struct {
	versionsMu  sync.Mutex
	versionsMap map[nodeInfo]int32

	metricsOnce sync.Once

	platformNames []string

	metricNumMachines             prometheus.Gauge
	metricNumConnectedMachines    prometheus.Gauge
	metricNumMachinesPerVersion   *prometheus.Desc
	metricMachinePlatforms        *prometheus.GaugeVec
	metricMachineSecureBootStatus *prometheus.GaugeVec
	metricMachineUKIStatus        *prometheus.GaugeVec
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
		{
			Namespace: resources.InfraProviderNamespace,
			Type:      infra.InfraMachineType,
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
			[]string{"talos_version", "cluster", "connected"},
			nil,
		)

		ctrl.metricMachinePlatforms = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "omni_machine_platforms",
			Help: "Number of machines in the instance by platform.",
		}, []string{"platform"})

		ctrl.metricMachineSecureBootStatus = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "omni_machine_secure_boot_status",
			Help: "Number of machines in the instance by secure boot status.",
		}, []string{"enabled"})

		ctrl.metricMachineUKIStatus = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "omni_machine_uki_status",
			Help: "Number of machines in the instance by UKI (Unified Kernel Image) status.",
		}, []string{"booted_with_uki"})

		allPlatforms := platforms.CloudPlatforms()
		allPlatforms = append(allPlatforms, platforms.MetalPlatform())

		ctrl.platformNames = make([]string, 0, len(allPlatforms))
		for _, p := range allPlatforms {
			ctrl.platformNames = append(ctrl.platformNames, p.Name)
		}
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

		pendingInfraMachines, err := safe.ReaderListAll[*infra.Machine](ctx, r, state.WithLabelQuery(resource.LabelExists(omni.LabelMachinePendingAccept)))
		if err != nil {
			return err
		}

		pendingMachines := pendingInfraMachines.Len()

		machineStatuses, err := safe.ReaderListAll[*omni.MachineStatus](ctx, r)
		if err != nil {
			return err
		}

		metricsSpec := ctrl.gatherMetrics(machineStatuses.All(), pendingMachines)

		if err = safe.WriterModify(ctx, r, omni.NewMachineStatusMetrics(omni.MachineStatusMetricsID),
			func(res *omni.MachineStatusMetrics) error {
				res.TypedSpec().Value = metricsSpec

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

func (ctrl *MachineStatusMetricsController) gatherMetrics(statuses iter.Seq[*omni.MachineStatus], numPendingMachines int) *specs.MachineStatusMetricsSpec {
	platformMetrics := make(map[string]uint32, len(ctrl.platformNames))
	for _, p := range ctrl.platformNames {
		platformMetrics[p] = 0
	}

	const statusUnknown = "unknown"

	secureBootStatusMetrics := map[string]uint32{
		statusUnknown: 0,
		"true":        0,
		"false":       0,
	}
	ukiMetrics := map[string]uint32{
		statusUnknown: 0,
		"true":        0,
		"false":       0,
	}

	var machines, connectedMachines, allocatedMachines int

	ctrl.versionsMu.Lock()
	ctrl.versionsMap = map[nodeInfo]int32{}

	for ms := range statuses {
		machines++

		if ms.TypedSpec().Value.Connected {
			connectedMachines++
		}

		if ms.TypedSpec().Value.TalosVersion != "" {
			ctrl.versionsMap[nodeInfo{
				talosVersion: ms.TypedSpec().Value.TalosVersion,
				cluster:      ms.TypedSpec().Value.Cluster,
				connected:    ms.TypedSpec().Value.Connected,
			}]++
		}

		if ms.TypedSpec().Value.Cluster != "" {
			allocatedMachines++
		}

		platform := ms.TypedSpec().Value.PlatformMetadata.GetPlatform()
		if platform != "" {
			platformMetrics[platform]++
		}

		securityState := ms.TypedSpec().Value.SecurityState
		if securityState != nil {
			secureBootStatusMetrics[strconv.FormatBool(securityState.SecureBoot)]++
			ukiMetrics[strconv.FormatBool(securityState.BootedWithUki)]++
		} else {
			secureBootStatusMetrics[statusUnknown]++
			ukiMetrics[statusUnknown]++
		}
	}

	ctrl.versionsMu.Unlock()

	ctrl.metricNumMachines.Set(float64(machines))
	ctrl.metricNumConnectedMachines.Set(float64(connectedMachines))

	for key, num := range platformMetrics {
		ctrl.metricMachinePlatforms.WithLabelValues(key).Set(float64(num))
	}

	for key, num := range secureBootStatusMetrics {
		ctrl.metricMachineSecureBootStatus.WithLabelValues(key).Set(float64(num))
	}

	for key, num := range ukiMetrics {
		ctrl.metricMachineUKIStatus.WithLabelValues(key).Set(float64(num))
	}

	return &specs.MachineStatusMetricsSpec{
		ConnectedMachinesCount:  uint32(connectedMachines),
		RegisteredMachinesCount: uint32(machines),
		AllocatedMachinesCount:  uint32(allocatedMachines),
		PendingMachinesCount:    uint32(numPendingMachines),
		Platforms:               platformMetrics,
		SecureBootStatus:        secureBootStatusMetrics,
		UkiStatus:               ukiMetrics,
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

	for info, count := range ctrl.versionsMap {
		ch <- prometheus.MustNewConstMetric(ctrl.metricNumMachinesPerVersion, prometheus.GaugeValue, float64(count), info.talosVersion, info.cluster, strconv.FormatBool(info.connected))
	}

	ctrl.versionsMu.Unlock()

	ctrl.metricNumMachines.Collect(ch)
	ctrl.metricNumConnectedMachines.Collect(ch)
	ctrl.metricMachinePlatforms.Collect(ch)
	ctrl.metricMachineSecureBootStatus.Collect(ch)
	ctrl.metricMachineUKIStatus.Collect(ch)
}

var _ prometheus.Collector = &MachineStatusMetricsController{}
