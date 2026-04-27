// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"fmt"
	"iter"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/blang/semver/v4"
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/siderolabs/talos/pkg/machinery/platforms"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
)

type nodeInfo struct {
	talosVersion string
	cluster      string
	connected    bool
}

const (
	// NonImageFactoryNotificationEnabled controls whether a notification is shown for non-ImageFactory machines.
	// This will be flipped to true in a future release.
	NonImageFactoryNotificationEnabled = false

	// UnsupportedTalosVersionNotificationEnabled controls whether notifications are shown for unsupported Talos versions.
	UnsupportedTalosVersionNotificationEnabled = true

	talosVersionSupportPolicyDocsURL = "https://docs.siderolabs.com/omni/getting-started/talos-version-support-policy"

	nonImageFactoryNotificationTitle = "Non-ImageFactory Machines Detected"
	nonImageFactoryNotificationBody  = "%d machine(s) were provisioned without ImageFactory." +
		" Omni will refuse to start with non-ImageFactory machines in a future release." +
		" Please re-provision them using ImageFactory."

	approachingTalosEndOfSupportNotificationTitle = "Talos Version Approaching End of Support"
	approachingTalosEndOfSupportNotificationBody  = "%d machine(s) are running a Talos version that will lose Omni support soon." +
		" The minimum supported version is %s. Please upgrade: " + talosVersionSupportPolicyDocsURL

	talosVersionEndOfSupportNotificationTitle = "Unsupported Talos Version Detected"
	talosVersionEndOfSupportNotificationBody  = "%d machine(s) are running unsupported Talos versions (below %s)." +
		" Please upgrade immediately: " + talosVersionSupportPolicyDocsURL
)

// NewMachineStatusMetricsController creates a new MachineStatusMetricsController.
func NewMachineStatusMetricsController(maxRegisteredMachines uint32) *MachineStatusMetricsController {
	return &MachineStatusMetricsController{
		maxRegisteredMachines: maxRegisteredMachines,
	}
}

// MachineStatusMetricsController provides metrics based on ClusterStatus.
//
//nolint:govet
type MachineStatusMetricsController struct {
	versionsMu  sync.Mutex
	versionsMap map[nodeInfo]int32

	metricsOnce sync.Once

	maxRegisteredMachines uint32

	platformNames []string

	metricNumMachines                                    prometheus.Gauge
	metricNumConnectedMachines                           prometheus.Gauge
	metricNumInvalidSchematicMachines                    prometheus.Gauge
	metricNumApproachingTalosVersionEndOfSupportMachines prometheus.Gauge
	metricNumTalosVersionEndOfSupportMachines            prometheus.Gauge
	metricNumMachinesPerVersion                          *prometheus.Desc
	metricMachinePlatforms                               *prometheus.GaugeVec
	metricMachineSecureBootStatus                        *prometheus.GaugeVec
	metricMachineUKIStatus                               *prometheus.GaugeVec
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
		{
			Type: omni.NotificationType,
			Kind: controller.OutputShared,
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

		ctrl.metricNumInvalidSchematicMachines = prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "omni_machines_invalid_schematic",
			Help: "Number of machines in the instance that were provisioned without using ImageFactory.",
		})

		ctrl.metricNumApproachingTalosVersionEndOfSupportMachines = prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "omni_machines_approaching_talos_version_end_of_support",
			Help: "Number of machines running a Talos version at or near the minimum supported version.",
		})

		ctrl.metricNumTalosVersionEndOfSupportMachines = prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "omni_machines_talos_version_end_of_support",
			Help: "Number of machines running a Talos version below the minimum supported version.",
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

		if ctrl.maxRegisteredMachines > 0 {
			if err = ctrl.reconcileRegistrationLimitNotification(ctx, r, metricsSpec); err != nil {
				return err
			}
		}

		if NonImageFactoryNotificationEnabled {
			if err = ctrl.reconcileNonImageFactoryDeprecationNotification(ctx, r, metricsSpec); err != nil {
				return err
			}
		}

		if UnsupportedTalosVersionNotificationEnabled {
			if err = ctrl.reconcileApproachingTalosVersionEndOfSupportNotification(ctx, r, metricsSpec); err != nil {
				return err
			}

			if err = ctrl.reconcileTalosVersionEndOfSupportNotification(ctx, r, metricsSpec); err != nil {
				return err
			}
		}

		select {
		case <-ctx.Done():
			return nil
		case <-time.After(10 * time.Second): // don't reconcile too often, as metrics are not scraped that often
		}
	}
}

func (ctrl *MachineStatusMetricsController) reconcileRegistrationLimitNotification(ctx context.Context, r controller.Runtime, metricsSpec *specs.MachineStatusMetricsSpec) error {
	if metricsSpec.RegistrationLimitReached {
		return safe.WriterModify(ctx, r, omni.NewNotification(omni.NotificationMachineRegistrationLimitID),
			func(res *omni.Notification) error {
				res.TypedSpec().Value.Title = "Machine Registration Limit Reached"
				res.TypedSpec().Value.Body = fmt.Sprintf(
					"%d/%d machines registered. New machines will be rejected.",
					metricsSpec.RegisteredMachinesCount, metricsSpec.RegisteredMachinesLimit)
				res.TypedSpec().Value.Type = specs.NotificationSpec_WARNING

				return nil
			},
		)
	}

	_, err := helpers.TeardownAndDestroy(ctx, r, omni.NewNotification(omni.NotificationMachineRegistrationLimitID).Metadata())

	return err
}

func (ctrl *MachineStatusMetricsController) reconcileNonImageFactoryDeprecationNotification(ctx context.Context, r controller.Runtime, metricsSpec *specs.MachineStatusMetricsSpec) error {
	invalidSchematicCount := int(metricsSpec.InvalidSchematicMachinesCount)
	if invalidSchematicCount > 0 {
		return safe.WriterModify(ctx, r, omni.NewNotification(omni.NotificationNonImageFactoryMachinesID),
			func(res *omni.Notification) error {
				res.TypedSpec().Value.Title = nonImageFactoryNotificationTitle
				res.TypedSpec().Value.Body = fmt.Sprintf(nonImageFactoryNotificationBody, invalidSchematicCount)
				res.TypedSpec().Value.Type = specs.NotificationSpec_WARNING

				return nil
			},
		)
	}

	_, err := helpers.TeardownAndDestroy(ctx, r, omni.NewNotification(omni.NotificationNonImageFactoryMachinesID).Metadata())

	return err
}

func (ctrl *MachineStatusMetricsController) reconcileApproachingTalosVersionEndOfSupportNotification(ctx context.Context, r controller.Runtime, metricsSpec *specs.MachineStatusMetricsSpec) error {
	count := int(metricsSpec.ApproachingTalosVersionEndOfSupportMachinesCount)
	if count > 0 {
		return safe.WriterModify(ctx, r, omni.NewNotification(omni.NotificationApproachingTalosVersionEndOfSupportID),
			func(res *omni.Notification) error {
				res.TypedSpec().Value.Title = approachingTalosEndOfSupportNotificationTitle
				res.TypedSpec().Value.Body = fmt.Sprintf(approachingTalosEndOfSupportNotificationBody, count, constants.MinTalosVersion)
				res.TypedSpec().Value.Type = specs.NotificationSpec_WARNING

				return nil
			},
		)
	}

	_, err := helpers.TeardownAndDestroy(ctx, r, omni.NewNotification(omni.NotificationApproachingTalosVersionEndOfSupportID).Metadata())

	return err
}

func (ctrl *MachineStatusMetricsController) reconcileTalosVersionEndOfSupportNotification(ctx context.Context, r controller.Runtime, metricsSpec *specs.MachineStatusMetricsSpec) error {
	count := int(metricsSpec.TalosVersionEndOfSupportMachinesCount)
	if count > 0 {
		return safe.WriterModify(ctx, r, omni.NewNotification(omni.NotificationTalosVersionEndOfSupportID),
			func(res *omni.Notification) error {
				res.TypedSpec().Value.Title = talosVersionEndOfSupportNotificationTitle
				res.TypedSpec().Value.Body = fmt.Sprintf(talosVersionEndOfSupportNotificationBody, count, constants.MinTalosVersion)
				res.TypedSpec().Value.Type = specs.NotificationSpec_WARNING

				return nil
			},
		)
	}

	_, err := helpers.TeardownAndDestroy(ctx, r, omni.NewNotification(omni.NotificationTalosVersionEndOfSupportID).Metadata())

	return err
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

	minTalosVer := semver.MustParse(constants.MinTalosVersion)
	// Machines at MinTalosVersion or 1 minor above are "approaching talos end of support"
	approachingThreshold := semver.Version{Major: minTalosVer.Major, Minor: minTalosVer.Minor + 2}

	var (
		machines, connectedMachines, allocatedMachines, invalidSchematicMachines int
		approachingEndOfSupportMachines, endOfSupportTalosVersionMachines        int
	)

	ctrl.versionsMu.Lock()
	ctrl.versionsMap = map[nodeInfo]int32{}

	for ms := range statuses {
		machines++

		if ms.TypedSpec().Value.Connected {
			connectedMachines++
		}

		talosVersion := ms.TypedSpec().Value.TalosVersion

		if talosVersion != "" {
			ctrl.versionsMap[nodeInfo{
				talosVersion: talosVersion,
				cluster:      ms.TypedSpec().Value.Cluster,
				connected:    ms.TypedSpec().Value.Connected,
			}]++

			if ver, err := semver.ParseTolerant(strings.TrimLeft(talosVersion, "v")); err == nil {
				ver.Pre = nil

				switch {
				case ver.LT(minTalosVer):
					endOfSupportTalosVersionMachines++
				case ver.LT(approachingThreshold):
					approachingEndOfSupportMachines++
				}
			}
		}

		if ms.TypedSpec().Value.Cluster != "" {
			allocatedMachines++
		}

		if ms.TypedSpec().Value.SchematicReady() && ms.TypedSpec().Value.GetSchematic().GetInvalid() {
			invalidSchematicMachines++
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
	ctrl.metricNumInvalidSchematicMachines.Set(float64(invalidSchematicMachines))
	ctrl.metricNumApproachingTalosVersionEndOfSupportMachines.Set(float64(approachingEndOfSupportMachines))
	ctrl.metricNumTalosVersionEndOfSupportMachines.Set(float64(endOfSupportTalosVersionMachines))

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
		ConnectedMachinesCount:        uint32(connectedMachines),
		RegisteredMachinesCount:       uint32(machines),
		AllocatedMachinesCount:        uint32(allocatedMachines),
		PendingMachinesCount:          uint32(numPendingMachines),
		Platforms:                     platformMetrics,
		SecureBootStatus:              secureBootStatusMetrics,
		UkiStatus:                     ukiMetrics,
		RegisteredMachinesLimit:       ctrl.maxRegisteredMachines,
		RegistrationLimitReached:      ctrl.maxRegisteredMachines > 0 && uint32(machines) >= ctrl.maxRegisteredMachines,
		InvalidSchematicMachinesCount: uint32(invalidSchematicMachines),
		ApproachingTalosVersionEndOfSupportMachinesCount: uint32(approachingEndOfSupportMachines),
		TalosVersionEndOfSupportMachinesCount:            uint32(endOfSupportTalosVersionMachines),
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
	ctrl.metricNumInvalidSchematicMachines.Collect(ch)
	ctrl.metricNumApproachingTalosVersionEndOfSupportMachines.Collect(ch)
	ctrl.metricNumTalosVersionEndOfSupportMachines.Collect(ch)
	ctrl.metricMachinePlatforms.Collect(ch)
	ctrl.metricMachineSecureBootStatus.Collect(ch)
	ctrl.metricMachineUKIStatus.Collect(ch)
}

var _ prometheus.Collector = &MachineStatusMetricsController{}
