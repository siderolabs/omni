// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package metrics

import (
	"context"
	"fmt"
	"iter"
	"strconv"
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
	"github.com/siderolabs/omni/client/pkg/siderolink"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
)

var talosVersionFloor = semver.MustParse(constants.TalosVersionFloor)

type nodeInfo struct {
	talosVersion string
	cluster      string
	connected    bool
}

type invalidSchematicInfo struct {
	connected bool
	allocated bool
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

	talosVersionBelowFloorNotificationTitle = "Talos Version Too Old to Connect"
	talosVersionBelowFloorNotificationBody  = "%d machine(s) are running Talos versions below %s." +
		" These machines will not be able to communicate with Omni in an upcoming release. Please upgrade: " + talosVersionSupportPolicyDocsURL

	talosVersionBelowFloorDisconnectedNotificationTitle = "Machines Disconnected for Talos Version"
	talosVersionBelowFloorDisconnectedNotificationBody  = "%d machine(s) are running Talos versions below %s and are disconnected from Omni." +
		" Remove them, or reinstall them with a supported Talos version: " + talosVersionSupportPolicyDocsURL
)

// NewMachineStatusMetricsController creates a new MachineStatusMetricsController.
func NewMachineStatusMetricsController(maxRegisteredMachines uint32, disconnectBelowTalosVersionFloor bool) *MachineStatusMetricsController {
	return &MachineStatusMetricsController{
		maxRegisteredMachines:            maxRegisteredMachines,
		disconnectBelowTalosVersionFloor: disconnectBelowTalosVersionFloor,
	}
}

// MachineStatusMetricsController provides metrics based on ClusterStatus.
//
//nolint:govet
type MachineStatusMetricsController struct {
	versionsMu  sync.Mutex
	versionsMap map[nodeInfo]int32
	coresMap    map[uint32]int32

	metricsOnce sync.Once

	maxRegisteredMachines uint32

	disconnectBelowTalosVersionFloor bool

	platformNames []string

	metricNumMachines                                    prometheus.Gauge
	metricNumConnectedMachines                           prometheus.Gauge
	metricNumInvalidSchematicMachines                    *prometheus.GaugeVec
	metricNumApproachingTalosVersionEndOfSupportMachines prometheus.Gauge
	metricNumCores                                       prometheus.Gauge
	metricNumTalosVersionEndOfSupportMachines            prometheus.Gauge
	metricNumTalosVersionBelowFloorMachines              prometheus.Gauge
	metricNumMachinesPerVersion                          *prometheus.Desc
	metricNumMachinesPerCores                            *prometheus.Desc
	metricMachinePlatforms                               *prometheus.GaugeVec
	metricMachineSecureBootStatus                        *prometheus.GaugeVec
	metricMachineUKIStatus                               *prometheus.GaugeVec
	metricMachineJoinConfig                              *prometheus.GaugeVec
	metricMachineTrustedRoots                            *prometheus.GaugeVec
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

		ctrl.metricNumCores = prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "omni_machines_cores",
			Help: "Number of CPU cores in the instance.",
		})

		ctrl.metricNumConnectedMachines = prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "omni_connected_machines",
			Help: "Number of machines in the instance that are connected.",
		})

		ctrl.metricNumInvalidSchematicMachines = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "omni_machines_invalid_schematic",
			Help: "Number of machines in the instance that were provisioned without using ImageFactory.",
		}, []string{"connected", "allocated"})

		ctrl.setInvalidSchematicMetrics(newInvalidSchematicMetrics())

		ctrl.metricNumApproachingTalosVersionEndOfSupportMachines = prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "omni_machines_approaching_talos_version_end_of_support",
			Help: "Number of machines running a Talos version at or near the minimum supported version.",
		})

		ctrl.metricNumTalosVersionEndOfSupportMachines = prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "omni_machines_talos_version_end_of_support",
			Help: "Number of machines running a Talos version below the minimum supported version.",
		})

		ctrl.metricNumTalosVersionBelowFloorMachines = prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "omni_machines_talos_version_below_floor",
			Help: "Number of machines running a Talos version below the lowest version which can connect to Omni.",
		})

		ctrl.metricNumMachinesPerVersion = prometheus.NewDesc(
			"omni_machines_version",
			"Number of machines in the instance by version.",
			[]string{"talos_version", "cluster", "connected"},
			nil,
		)

		ctrl.metricNumMachinesPerCores = prometheus.NewDesc(
			"omni_machine_cores",
			"Number of machines in the instance by CPU core count.",
			[]string{"cores"},
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

		ctrl.metricMachineJoinConfig = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "omni_machine_siderolink_join_config",
			Help: "Number of machines in the instance by where their SideroLink join config is.",
		}, []string{"location"})

		ctrl.metricMachineTrustedRoots = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "omni_machine_schematic_trusted_roots",
			Help: "Number of machines in the instance by whether the schematic they run carries Omni's trusted roots.",
		}, []string{"present"})

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

		if err = safe.WriterModify(
			ctx, r, omni.NewMachineStatusMetrics(omni.MachineStatusMetricsID),
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

		if err = ctrl.reconcileTalosVersionNotifications(ctx, r, metricsSpec); err != nil {
			return err
		}

		select {
		case <-ctx.Done():
			return nil
		case <-time.After(10 * time.Second): // don't reconcile too often, as metrics are not scraped that often
		}
	}
}

func (ctrl *MachineStatusMetricsController) reconcileTalosVersionNotifications(ctx context.Context, r controller.Runtime, metricsSpec *specs.MachineStatusMetricsSpec) error {
	if UnsupportedTalosVersionNotificationEnabled {
		if err := ctrl.reconcileApproachingTalosVersionEndOfSupportNotification(ctx, r, metricsSpec); err != nil {
			return err
		}

		if err := ctrl.reconcileTalosVersionEndOfSupportNotification(ctx, r, metricsSpec); err != nil {
			return err
		}
	}

	return ctrl.reconcileTalosVersionBelowFloorNotification(ctx, r, metricsSpec)
}

func (ctrl *MachineStatusMetricsController) reconcileRegistrationLimitNotification(ctx context.Context, r controller.Runtime, metricsSpec *specs.MachineStatusMetricsSpec) error {
	if metricsSpec.RegistrationLimitReached {
		return safe.WriterModify(
			ctx, r, omni.NewNotification(omni.NotificationMachineRegistrationLimitID),
			func(res *omni.Notification) error {
				res.TypedSpec().Value.Title = "Machine Registration Limit Reached"
				res.TypedSpec().Value.Body = fmt.Sprintf(
					"%d/%d machines registered. New machines will be rejected.",
					metricsSpec.RegisteredMachinesCount, metricsSpec.RegisteredMachinesLimit,
				)
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
		return safe.WriterModify(
			ctx, r, omni.NewNotification(omni.NotificationNonImageFactoryMachinesID),
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
		return safe.WriterModify(
			ctx, r, omni.NewNotification(omni.NotificationApproachingTalosVersionEndOfSupportID),
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
		return safe.WriterModify(
			ctx, r, omni.NewNotification(omni.NotificationTalosVersionEndOfSupportID),
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

func (ctrl *MachineStatusMetricsController) reconcileTalosVersionBelowFloorNotification(ctx context.Context, r controller.Runtime, metricsSpec *specs.MachineStatusMetricsSpec) error {
	id, staleID := omni.NotificationTalosVersionBelowFloorID, omni.NotificationTalosVersionBelowFloorDisconnectedID
	if ctrl.disconnectBelowTalosVersionFloor {
		id, staleID = staleID, id
	}

	if _, err := helpers.TeardownAndDestroy(ctx, r, omni.NewNotification(staleID).Metadata()); err != nil {
		return err
	}

	count := int(metricsSpec.TalosVersionBelowFloorMachinesCount)
	if count == 0 {
		_, err := helpers.TeardownAndDestroy(ctx, r, omni.NewNotification(id).Metadata())

		return err
	}

	return safe.WriterModify(ctx, r, omni.NewNotification(id), func(res *omni.Notification) error {
		if ctrl.disconnectBelowTalosVersionFloor {
			res.TypedSpec().Value.Title = talosVersionBelowFloorDisconnectedNotificationTitle
			res.TypedSpec().Value.Body = fmt.Sprintf(talosVersionBelowFloorDisconnectedNotificationBody, count, constants.TalosVersionFloor)
			res.TypedSpec().Value.Type = specs.NotificationSpec_WARNING

			return nil
		}

		res.TypedSpec().Value.Title = talosVersionBelowFloorNotificationTitle
		res.TypedSpec().Value.Body = fmt.Sprintf(talosVersionBelowFloorNotificationBody, count, constants.TalosVersionFloor)
		res.TypedSpec().Value.Type = specs.NotificationSpec_ERROR
		res.TypedSpec().Value.Undismissible = true

		return nil
	})
}

type talosVersionCounts struct {
	endOfSupport int
	approaching  int
	belowFloor   int
}

func (counts *talosVersionCounts) add(talosVersion string, minTalosVer, approachingThreshold semver.Version) {
	ver, err := siderolink.ParseTalosVersion(talosVersion)
	if err != nil {
		return // the machine did not report its version yet
	}

	switch {
	case ver.LT(minTalosVer):
		counts.endOfSupport++
	case ver.LT(approachingThreshold):
		counts.approaching++
	}

	if ver.LT(talosVersionFloor) {
		counts.belowFloor++
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

	joinConfigMetrics := map[string]uint32{
		joinConfigKernelArgs:    0,
		joinConfigEmbedded:      0,
		joinConfigMachineConfig: 0,
		joinConfigUnknown:       0,
	}

	trustedRootsMetrics := map[string]uint32{
		statusUnknown: 0,
		"true":        0,
		"false":       0,
	}

	invalidSchematicMetrics := newInvalidSchematicMetrics()

	minTalosVer := semver.MustParse(constants.MinTalosVersion)
	// Machines at MinTalosVersion or 1 minor above are "approaching talos end of support"
	approachingThreshold := semver.Version{Major: minTalosVer.Major, Minor: minTalosVer.Minor + 2}

	var (
		machines, connectedMachines, allocatedMachines, invalidSchematicMachines int
		talosVersions                                                            talosVersionCounts
	)

	var cores uint64

	ctrl.versionsMu.Lock()
	ctrl.versionsMap = map[nodeInfo]int32{}
	ctrl.coresMap = map[uint32]int32{}

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

			talosVersions.add(talosVersion, minTalosVer, approachingThreshold)
		}

		if ms.TypedSpec().Value.Cluster != "" {
			allocatedMachines++
		}

		if ms.TypedSpec().Value.SchematicReady() && ms.TypedSpec().Value.GetSchematic().GetInvalid() {
			invalidSchematicMachines++

			invalidSchematicMetrics[invalidSchematicInfo{
				connected: ms.TypedSpec().Value.Connected,
				allocated: ms.TypedSpec().Value.Cluster != "",
			}]++
		}

		platform := ms.TypedSpec().Value.PlatformMetadata.GetPlatform()
		if platform != "" {
			platformMetrics[platform]++
		}

		var machineCores uint32

		for _, proc := range ms.TypedSpec().Value.GetHardware().GetProcessors() {
			machineCores += proc.GetCoreCount()
		}

		cores += uint64(machineCores)
		ctrl.coresMap[machineCores]++

		joinConfigMetrics[joinConfigLocation(ms)]++
		trustedRootsMetrics[trustedRootsPresent(ms)]++

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
	ctrl.metricNumApproachingTalosVersionEndOfSupportMachines.Set(float64(talosVersions.approaching))
	ctrl.metricNumTalosVersionEndOfSupportMachines.Set(float64(talosVersions.endOfSupport))
	ctrl.metricNumTalosVersionBelowFloorMachines.Set(float64(talosVersions.belowFloor))
	ctrl.setInvalidSchematicMetrics(invalidSchematicMetrics)

	setByLabel(ctrl.metricMachinePlatforms, platformMetrics)
	setByLabel(ctrl.metricMachineSecureBootStatus, secureBootStatusMetrics)
	setByLabel(ctrl.metricMachineUKIStatus, ukiMetrics)
	setByLabel(ctrl.metricMachineJoinConfig, joinConfigMetrics)
	setByLabel(ctrl.metricMachineTrustedRoots, trustedRootsMetrics)

	ctrl.metricNumCores.Set(float64(cores))

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
		ApproachingTalosVersionEndOfSupportMachinesCount: uint32(talosVersions.approaching),
		TalosVersionEndOfSupportMachinesCount:            uint32(talosVersions.endOfSupport),
		TalosVersionBelowFloorMachinesCount:              uint32(talosVersions.belowFloor),
		CoresCount:                                       cores,
	}
}

// setByLabel sets the gauge of each label value to its count.
func setByLabel(vec *prometheus.GaugeVec, counts map[string]uint32) {
	for key, num := range counts {
		vec.WithLabelValues(key).Set(float64(num))
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

	for machineCores, count := range ctrl.coresMap {
		ch <- prometheus.MustNewConstMetric(ctrl.metricNumMachinesPerCores, prometheus.GaugeValue, float64(count), strconv.FormatUint(uint64(machineCores), 10))
	}

	ctrl.versionsMu.Unlock()

	ctrl.metricNumMachines.Collect(ch)
	ctrl.metricNumCores.Collect(ch)
	ctrl.metricNumConnectedMachines.Collect(ch)
	ctrl.metricNumInvalidSchematicMachines.Collect(ch)
	ctrl.metricNumApproachingTalosVersionEndOfSupportMachines.Collect(ch)
	ctrl.metricNumTalosVersionEndOfSupportMachines.Collect(ch)
	ctrl.metricNumTalosVersionBelowFloorMachines.Collect(ch)
	ctrl.metricMachinePlatforms.Collect(ch)
	ctrl.metricMachineSecureBootStatus.Collect(ch)
	ctrl.metricMachineUKIStatus.Collect(ch)
	ctrl.metricMachineJoinConfig.Collect(ch)
	ctrl.metricMachineTrustedRoots.Collect(ch)
}

func newInvalidSchematicMetrics() map[invalidSchematicInfo]int {
	return map[invalidSchematicInfo]int{
		{connected: false, allocated: false}: 0,
		{connected: false, allocated: true}:  0,
		{connected: true, allocated: false}:  0,
		{connected: true, allocated: true}:   0,
	}
}

func (ctrl *MachineStatusMetricsController) setInvalidSchematicMetrics(invalidSchematicMetrics map[invalidSchematicInfo]int) {
	for info, num := range invalidSchematicMetrics {
		ctrl.metricNumInvalidSchematicMachines.WithLabelValues(strconv.FormatBool(info.connected), strconv.FormatBool(info.allocated)).Set(float64(num))
	}
}

var _ prometheus.Collector = &MachineStatusMetricsController{}
