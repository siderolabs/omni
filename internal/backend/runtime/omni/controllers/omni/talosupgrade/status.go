// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package talosupgrade contains controllers related to Talos upgrade process.
package talosupgrade

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/gertd/go-pluralize"
	"github.com/siderolabs/gen/xerrors"
	"github.com/siderolabs/gen/xiter"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-pointer"
	"go.uber.org/zap"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/kubernetes"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/mappers"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/talos"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/machineconfig"
)

// KubernetesRuntime provides kubernetes cluster access capabilities.
type KubernetesRuntime interface {
	GetClient(ctx context.Context, cluster string) (*kubernetes.Client, error)
}

// TalosUpgradeStatusController manages TalosUpgradeStatus performing a Talos upgrade.
//
// TalosUpgradeStatusController manages Talos OS (Talos) version upgrades in the cluster.
type TalosUpgradeStatusController struct {
	*qtransform.QController[*omni.Cluster, *omni.TalosUpgradeStatus]
	kubernetesRuntime KubernetesRuntime
}

// TalosUpgradeStatusControllerName is the name of TalosUpgradeStatusController.
const TalosUpgradeStatusControllerName = "TalosUpgradeStatusController"

type outdatedMachines struct {
	all       map[string]*omni.ClusterMachine
	upgrading []string
}

// NewStatusController initializes TalosUpgradeStatusController.
func NewStatusController(kubernetesRuntime KubernetesRuntime) *TalosUpgradeStatusController {
	ctrl := &TalosUpgradeStatusController{
		kubernetesRuntime: kubernetesRuntime,
	}

	ctrl.QController = qtransform.NewQController(
		qtransform.Settings[*omni.Cluster, *omni.TalosUpgradeStatus]{
			Name: TalosUpgradeStatusControllerName,
			MapMetadataFunc: func(cluster *omni.Cluster) *omni.TalosUpgradeStatus {
				return omni.NewTalosUpgradeStatus(cluster.Metadata().ID())
			},
			UnmapMetadataFunc: func(upgradeStatus *omni.TalosUpgradeStatus) *omni.Cluster {
				return omni.NewCluster(upgradeStatus.Metadata().ID())
			},
			TransformExtraOutputFunc: func(ctx context.Context, r controller.ReaderWriter, logger *zap.Logger, cluster *omni.Cluster, upgradeStatus *omni.TalosUpgradeStatus) error {
				clusterMachines, err := safe.ReaderListAll[*omni.ClusterMachine](ctx, r, state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, cluster.Metadata().ID())))
				if err != nil {
					return err
				}

				outdatedMachines, err := ctrl.filterOutdated(ctx, r, cluster, clusterMachines)
				if err != nil {
					return err
				}

				if err := ctrl.reconcileStatus(ctx, r, clusterMachines, outdatedMachines, cluster, upgradeStatus); err != nil {
					return err
				}

				if err := ctrl.updateRolloutState(ctx, r, logger, outdatedMachines, cluster, upgradeStatus); err != nil {
					return err
				}

				if err := ctrl.reconcileTalosVersions(ctx, r, cluster, clusterMachines); err != nil {
					return err
				}

				return ctrl.reconcileUpgradeVersions(ctx, r, logger, cluster, upgradeStatus)
			},
			FinalizerRemovalExtraOutputFunc: func(ctx context.Context, r controller.ReaderWriter, _ *zap.Logger, cluster *omni.Cluster) error {
				clusterMachineTalosVersions, err := safe.ReaderListAll[*omni.ClusterMachineTalosVersion](ctx, r, state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, cluster.Metadata().ID())))
				if err != nil {
					return err
				}

				ready, err := helpers.TeardownAndDestroyAll(ctx, r, clusterMachineTalosVersions.Pointers())
				if err != nil {
					return err
				}

				if !ready {
					return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("cluster machine Talos versions are still tearing down")
				}

				ready, err = helpers.TeardownAndDestroy(ctx, r, omni.NewUpgradeRollout(cluster.Metadata().ID()).Metadata())
				if err != nil {
					return err
				}

				if !ready {
					return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("upgrade rollout %q tearing down", omni.NewUpgradeRollout(cluster.Metadata().ID()).Metadata().ID())
				}

				return nil
			},
		},
		qtransform.WithExtraMappedInput[*omni.TalosVersion](
			func(ctx context.Context, _ *zap.Logger, r controller.QRuntime, _ controller.ReducedResourceMetadata) ([]resource.Pointer, error) {
				// reconcile all cluster TalosUpgradeStatus on TalosVersion changes
				clusters, err := safe.ReaderListAll[*omni.Cluster](ctx, r)
				if err != nil {
					return nil, err
				}

				return slices.Collect(clusters.Pointers()), nil
			},
		),
		qtransform.WithExtraMappedInput[*omni.ClusterMachineStatus](
			mappers.MapByClusterLabel[*omni.Cluster](),
		),
		qtransform.WithExtraMappedInput[*omni.MachineSetNode](
			mappers.MapByClusterLabel[*omni.Cluster](),
		),
		qtransform.WithExtraMappedInput[*omni.ClusterMachine](
			mappers.MapByClusterLabel[*omni.Cluster](),
		),
		qtransform.WithExtraMappedInput[*omni.SchematicConfiguration](
			mappers.MapByClusterLabel[*omni.Cluster](),
		),
		qtransform.WithExtraMappedInput[*omni.MachinePendingUpdates](
			mappers.MapByClusterLabel[*omni.Cluster](),
		),
		qtransform.WithExtraMappedInput[*omni.ClusterMachineConfigStatus](
			mappers.MapByClusterLabel[*omni.Cluster](),
		),
		qtransform.WithExtraMappedInput[*omni.ClusterConfigVersion](
			qtransform.MapperNone(),
		),
		qtransform.WithExtraMappedInput[*omni.MachineSet](
			mappers.MapByClusterLabel[*omni.Cluster](),
		),
		qtransform.WithExtraMappedInput[*omni.KubernetesHealthCheck](
			mappers.MapByClusterLabel[*omni.Cluster](),
		),
		qtransform.WithExtraOutputs(
			controller.Output{
				Type: omni.ClusterMachineTalosVersionType,
				Kind: controller.OutputExclusive,
			},
		),
		qtransform.WithExtraOutputs(
			controller.Output{
				Type: omni.UpgradeRolloutType,
				Kind: controller.OutputExclusive,
			},
		),
	)

	return ctrl
}

//nolint:gocyclo,cyclop
func (ctrl *TalosUpgradeStatusController) reconcileStatus(ctx context.Context, r controller.ReaderWriter,
	clusterMachines safe.List[*omni.ClusterMachine], outdatedMachines *outdatedMachines,
	cluster *omni.Cluster, upgradeStatus *omni.TalosUpgradeStatus,
) error {
	machineSetNodes, err := safe.ReaderListAll[*omni.MachineSetNode](ctx, r, state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, cluster.Metadata().ID())))
	if err != nil {
		return err
	}

	machineSetNodeMap := xslices.ToMap(slices.Collect(machineSetNodes.All()), func(r *omni.MachineSetNode) (string, *omni.MachineSetNode) {
		return r.Metadata().ID(), r
	})

	talosVersion := cluster.TypedSpec().Value.TalosVersion

	// if not reverting to the previous successful version, perform pre-checks on each step
	versionMismatch := talosVersion != upgradeStatus.TypedSpec().Value.LastUpgradeVersion
	schematicUpdates := false

	pendingUpdates, err := safe.ReaderListAll[*omni.MachinePendingUpdates](ctx, r, state.WithLabelQuery(
		resource.LabelEqual(omni.LabelCluster, cluster.Metadata().ID()),
	))
	if err != nil {
		return err
	}

	var (
		pendingMachines       int
		lockedPendingMachines int
	)

	for pendingUpdate := range pendingUpdates.All() {
		if pendingUpdate.TypedSpec().Value.Upgrade == nil {
			continue
		}

		if pendingUpdate.TypedSpec().Value.Upgrade.FromSchematic == "" ||
			pendingUpdate.TypedSpec().Value.Upgrade.FromVersion == "" {
			continue
		}

		msn, ok := machineSetNodeMap[pendingUpdate.Metadata().ID()]
		if ok {
			_, locked := msn.Metadata().Annotations().Get(omni.MachineLocked)
			if locked {
				lockedPendingMachines++
			}
		}

		pendingMachines++

		if pendingUpdate.TypedSpec().Value.Upgrade.FromSchematic != pendingUpdate.TypedSpec().Value.Upgrade.ToSchematic {
			schematicUpdates = true
		}
	}

	// exit early if there are no pending machines and no machines currently upgrading, setting the status to 'done'
	if pendingMachines == 0 && len(outdatedMachines.all) == 0 {
		upgradeStatus.TypedSpec().Value.Phase = specs.TalosUpgradeStatusSpec_Done
		upgradeStatus.TypedSpec().Value.Status = ""
		upgradeStatus.TypedSpec().Value.Step = ""
		upgradeStatus.TypedSpec().Value.CurrentUpgradeVersion = ""
		upgradeStatus.TypedSpec().Value.LastUpgradeVersion = talosVersion

		return nil
	}

	if versionMismatch {
		// set the status to 'upgrading' if there is a version mismatch
		upgradeStatus.TypedSpec().Value.Phase = specs.TalosUpgradeStatusSpec_Upgrading
		upgradeStatus.TypedSpec().Value.CurrentUpgradeVersion = talosVersion
	} else {
		upgradeStatus.TypedSpec().Value.CurrentUpgradeVersion = ""
	}

	totalMachines := clusterMachines.Len()

	currentMachine := min(totalMachines-pendingMachines+1, totalMachines)

	switch {
	case !versionMismatch && schematicUpdates:
		upgradeStatus.TypedSpec().Value.Phase = specs.TalosUpgradeStatusSpec_UpdatingMachineSchematics

		fallthrough
	case versionMismatch || schematicUpdates:
		if pendingMachines == 0 {
			upgradeStatus.TypedSpec().Value.Status = "updating"
		} else {
			upgradeStatus.TypedSpec().Value.Status = fmt.Sprintf("updating machines %d/%d", currentMachine, totalMachines)
		}
	default:
		upgradeStatus.TypedSpec().Value.Phase = specs.TalosUpgradeStatusSpec_Reverting
		upgradeStatus.TypedSpec().Value.Status = fmt.Sprintf("reverting update %d/%d", currentMachine, totalMachines)
	}

	if len(outdatedMachines.upgrading) > 0 {
		upgradeStatus.TypedSpec().Value.Step = fmt.Sprintf(
			"current %s: %s", pluralize.NewClient().Pluralize("machine", len(outdatedMachines.upgrading), false),
			strings.Join(outdatedMachines.upgrading, ", "),
		)

		return nil
	}

	if lockedPendingMachines == pendingMachines && lockedPendingMachines > 0 {
		upgradeStatus.TypedSpec().Value.Step = "all machines are locked"
		upgradeStatus.TypedSpec().Value.Status = "waiting for machines to be unlocked"

		return nil
	}

	upgradeStatus.TypedSpec().Value.Step = "no machines are currently upgrading"

	return nil
}

//nolint:gocognit,gocyclo,cyclop
func (ctrl *TalosUpgradeStatusController) updateRolloutState(ctx context.Context, r controller.ReaderWriter, logger *zap.Logger,
	outdatedMachines *outdatedMachines, cluster *omni.Cluster, upgradeStatus *omni.TalosUpgradeStatus,
) error {
	clusterMachineStatuses, err := safe.ReaderListAll[*omni.ClusterMachineStatus](ctx, r, state.WithLabelQuery(
		resource.LabelEqual(omni.LabelCluster, cluster.Metadata().ID()),
	))
	if err != nil {
		return err
	}

	var (
		controlPlanesUpgrading bool
		notReadyMachinesCounts int32
	)

	for clusterMachineStatus := range clusterMachineStatuses.All() {
		if clusterMachineStatus.TypedSpec().Value.Stage != specs.ClusterMachineStatusSpec_RUNNING ||
			!clusterMachineStatus.TypedSpec().Value.Ready {
			notReadyMachinesCounts++
		}
	}

	for _, clusterMachine := range outdatedMachines.all {
		if _, ok := clusterMachine.Metadata().Labels().Get(omni.LabelControlPlaneRole); ok {
			controlPlanesUpgrading = true

			break
		}
	}

	// Gate the rollout on the cluster-wide healthchecks in between node upgrades: only while an upgrade is
	// actually in progress, when the cluster is otherwise healthy (all machines ready) and nothing is
	// currently upgrading. Reverts and clusters already up to date skip healthchecks entirely, so a broken
	// cluster can always be rolled back.
	healthChecks := healthCheckResult{ready: true}

	//nolint:exhaustive // healthchecks only gate the in-progress upgrade phases; other phases skip them
	switch upgradeStatus.TypedSpec().Value.Phase {
	case specs.TalosUpgradeStatusSpec_Upgrading, specs.TalosUpgradeStatusSpec_UpdatingMachineSchematics:
		if notReadyMachinesCounts == 0 && len(outdatedMachines.upgrading) == 0 {
			healthChecks, err = ctrl.runHealthChecks(ctx, r, logger, cluster)
			if err != nil {
				return err
			}
		}
	}

	if err = safe.WriterModify(ctx, r, omni.NewUpgradeRollout(cluster.Metadata().ID()), func(res *omni.UpgradeRollout) error {
		// Hold the rollout (no upgrade quota for any machine set) until the cluster passes its healthchecks,
		// surfacing the reason so it's visible while the upgrade is paused.
		if !healthChecks.ready {
			res.TypedSpec().Value.MachineSetsUpgradeQuota = nil
			upgradeStatus.TypedSpec().Value.Step = healthChecks.reason

			return nil
		}

		var machineSets safe.List[*omni.MachineSet]

		machineSets, err = safe.ReaderListAll[*omni.MachineSet](ctx, r, state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, cluster.Metadata().ID())))
		if err != nil {
			return err
		}

		upgradeQuotas := make(map[string]int32, machineSets.Len())

		for machineSet := range machineSets.All() {
			quota := int32(omni.GetParallelism(machineSet.TypedSpec().Value.UpgradeStrategy, machineSet.TypedSpec().Value.UpgradeStrategyConfig, 1))

			// if reverting, ignore not ready machines
			if upgradeStatus.TypedSpec().Value.Phase != specs.TalosUpgradeStatusSpec_Reverting {
				quota -= notReadyMachinesCounts
			}

			if quota <= 0 {
				continue
			}

			_, isControlPlane := machineSet.Metadata().Labels().Get(omni.LabelControlPlaneRole)
			if isControlPlane || !controlPlanesUpgrading {
				upgradeQuotas[machineSet.Metadata().ID()] = quota
			}
		}

		res.TypedSpec().Value.MachineSetsUpgradeQuota = upgradeQuotas

		if len(res.TypedSpec().Value.MachineSetsUpgradeQuota) == 0 && upgradeStatus.TypedSpec().Value.Phase != specs.TalosUpgradeStatusSpec_Done {
			upgradeStatus.TypedSpec().Value.Step = "waiting for the cluster to be ready"
		}

		return nil
	}); err != nil {
		return err
	}

	// KubernetesHealthChecks are evaluated against the workload cluster, whose state does not feed back into Omni's
	// resources, so nothing would re-trigger this controller once the cluster recovers. Requeue to re-check.
	if !healthChecks.ready {
		return controller.NewRequeueInterval(healthChecks.interval)
	}

	return nil
}

func (ctrl *TalosUpgradeStatusController) reconcileTalosVersions(ctx context.Context, r controller.ReaderWriter, cluster *omni.Cluster,
	clusterMachines safe.List[*omni.ClusterMachine],
) error {
	clusterMachineTalosVersions, err := safe.ReaderListAll[*omni.ClusterMachineTalosVersion](
		ctx, r,
		state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, cluster.Metadata().ID())),
	)
	if err != nil {
		return err
	}

	visited := make(map[string]struct{}, clusterMachines.Len())

	for clusterMachine := range clusterMachines.All() {
		if clusterMachine.Metadata().Phase() == resource.PhaseTearingDown {
			continue
		}

		var schematicConfiguration *omni.SchematicConfiguration

		schematicConfiguration, err = safe.ReaderGetByID[*omni.SchematicConfiguration](ctx, r, clusterMachine.Metadata().ID())
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return err
		}

		if schematicConfiguration.Metadata().Phase() == resource.PhaseTearingDown {
			continue
		}

		visited[schematicConfiguration.Metadata().ID()] = struct{}{}

		if schematicConfiguration.TypedSpec().Value.TalosVersion != cluster.TypedSpec().Value.TalosVersion {
			continue
		}

		if err = safe.WriterModify(ctx, r, omni.NewClusterMachineTalosVersion(schematicConfiguration.Metadata().ID()), func(res *omni.ClusterMachineTalosVersion) error {
			res.TypedSpec().Value.SchematicId = schematicConfiguration.TypedSpec().Value.SchematicId
			res.TypedSpec().Value.TalosVersion = schematicConfiguration.TypedSpec().Value.TalosVersion

			helpers.CopyAllLabels(schematicConfiguration, res)

			return nil
		}); err != nil {
			return err
		}
	}

	toDestroy := xiter.Filter(func(r resource.Pointer) bool {
		_, ok := visited[r.ID()]

		return !ok
	}, clusterMachineTalosVersions.Pointers())

	if _, err = helpers.TeardownAndDestroyAll(ctx, r, toDestroy); err != nil {
		return err
	}

	return nil
}

func (ctrl *TalosUpgradeStatusController) reconcileUpgradeVersions(
	ctx context.Context,
	r controller.ReaderWriter,
	logger *zap.Logger,
	cluster *omni.Cluster,
	upgradeStatus *omni.TalosUpgradeStatus,
) error {
	if upgradeStatus.TypedSpec().Value.Phase != specs.TalosUpgradeStatusSpec_Done {
		// upgrade is going, or failed, so clear the upgrade versions
		upgradeStatus.TypedSpec().Value.UpgradeVersions = nil

		return nil
	}

	clusterConfigVersion, err := safe.ReaderGetByID[*omni.ClusterConfigVersion](ctx, r, cluster.Metadata().ID())
	if err != nil {
		if state.IsNotFoundError(err) {
			return xerrors.NewTagged[qtransform.SkipReconcileTag](err)
		}

		return err
	}

	// upgrade is done, so calculate the upgrade versions
	upgradeStatus.TypedSpec().Value.UpgradeVersions, err = talos.CalculateUpgradeVersions(
		ctx, r, logger, clusterConfigVersion.TypedSpec().Value.Version, upgradeStatus.TypedSpec().Value.LastUpgradeVersion, cluster.TypedSpec().Value.KubernetesVersion,
	)

	return err
}

func (ctrl *TalosUpgradeStatusController) filterOutdated(ctx context.Context, r controller.ReaderWriter,
	cluster *omni.Cluster, clusterMachines safe.List[*omni.ClusterMachine],
) (*outdatedMachines, error) {
	res := &outdatedMachines{
		all: make(map[string]*omni.ClusterMachine, clusterMachines.Len()),
	}

	for clusterMachine := range clusterMachines.All() {
		schematicConfiguration, err := safe.ReaderGetByID[*omni.SchematicConfiguration](ctx, r, clusterMachine.Metadata().ID())
		if err != nil {
			// this means the machine is not yet fully added to the cluster
			if state.IsNotFoundError(err) {
				continue
			}

			return nil, err
		}

		clusterMachineConfigStatus, err := safe.ReaderGetByID[*omni.ClusterMachineConfigStatus](ctx, r, clusterMachine.Metadata().ID())
		if err != nil {
			// this means the machine is not yet fully added to the cluster
			if state.IsNotFoundError(err) {
				continue
			}

			return nil, err
		}

		// ignore machines with missing TalosVersion or SchematicId, as they are not fully configured yet
		// so it's not outdated, but rather in the process of being installed
		if clusterMachineConfigStatus.TypedSpec().Value.TalosVersion == "" || clusterMachineConfigStatus.TypedSpec().Value.SchematicId == "" {
			continue
		}

		if clusterMachine.Metadata().Finalizers().Has(machineconfig.UpgradeFinalizer) {
			res.all[clusterMachine.Metadata().ID()] = clusterMachine
			res.upgrading = append(res.upgrading, clusterMachine.Metadata().ID())

			continue
		}

		// mark the machine as outdated if the schematic was generated for the outdated Talos version
		// that happens async in the controller, so we need to make sure it's in sync before proceeding
		if schematicConfiguration.TypedSpec().Value.TalosVersion != cluster.TypedSpec().Value.TalosVersion {
			res.all[clusterMachine.Metadata().ID()] = clusterMachine

			continue
		}

		if clusterMachineConfigStatus.TypedSpec().Value.TalosVersion != schematicConfiguration.TypedSpec().Value.TalosVersion ||
			clusterMachineConfigStatus.TypedSpec().Value.SchematicId != schematicConfiguration.TypedSpec().Value.SchematicId {
			res.all[clusterMachine.Metadata().ID()] = clusterMachine
		}
	}

	return res, nil
}

const (
	// DefaultHealthCheckNamespace is the namespace a healthcheck job is created in when its manifest does not
	// specify one.
	DefaultHealthCheckNamespace = "default"
	// HealthCheckRunnerName is the value of the app.kubernetes.io/name label set on the healthcheck runner
	// jobs, also used to select them for cleanup.
	HealthCheckRunnerName = "omni-health-checker"
	// HealthCheckConfigHashAnnotation stores a hash of the healthcheck job manifest, so the job is recreated
	// when the manifest changes.
	HealthCheckConfigHashAnnotation = "omni.sidero.dev/healthcheck-config-hash"
)

// HealthCheckRunnerNamePrefix is prepended to a healthcheck's resource ID to derive the name of its runner
// job. The healthcheck ID is validated on creation so that the resulting name is a valid Kubernetes object
// name (see the KubernetesHealthCheck validation in the state validator).
const HealthCheckRunnerNamePrefix = "omni-healthcheck-"

const (
	// defaultHealthCheckInterval is how often a holding healthcheck is re-evaluated when it doesn't set its own.
	defaultHealthCheckInterval = 30 * time.Second
	// healthCheckOutputLimit is the maximum number of characters of a job failure reason surfaced in the status.
	healthCheckOutputLimit = 256
	// healthCheckJobRunningDetail is the status detail reported while a healthcheck job is still running.
	healthCheckJobRunningDetail = "waiting to complete"
)

// healthCheckRunnerSelector selects the healthcheck runner jobs for cleanup.
var healthCheckRunnerSelector = "app.kubernetes.io/name=" + HealthCheckRunnerName

// healthCheckResult is the outcome of running the cluster-wide healthchecks: whether the cluster is ready
// to proceed with the upgrade and, when it is not, a human-readable reason to surface in the upgrade status
// along with the interval after which the held rollout should re-check.
type healthCheckResult struct {
	reason   string
	interval time.Duration
	ready    bool
}

// healthCheckRunnerStatus is the result of evaluating a single healthcheck (or its runner pod): whether it is
// ready/passing and, when it isn't, a human-readable detail explaining why.
type healthCheckRunnerStatus struct {
	detail string
	ready  bool
}

// runHealthChecks runs all cluster-wide healthchecks once. A failing or still-running healthcheck holds the
// rollout; genuine errors (unreachable API, missing permissions, Omni state errors) are returned so the
// controller retries instead of being surfaced as a held state.
//
// Each healthcheck is executed in a runner pod that is created fresh on each attempt and deleted once it reaches a terminal state,
// so the next attempt runs a fresh check. A non-nil error is only returned for unexpected Kubernetes API or manifest-parsing errors.
func (ctrl *TalosUpgradeStatusController) runHealthChecks(ctx context.Context, r controller.Reader, logger *zap.Logger, cluster *omni.Cluster) (healthCheckResult, error) {
	healthchecks, err := safe.ReaderListAll[*omni.KubernetesHealthCheck](ctx, r, state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, cluster.Metadata().ID())))
	if err != nil {
		return healthCheckResult{}, err
	}

	if healthchecks.Len() == 0 {
		// tear down any runner pods left over from previously-removed healthchecks
		ctrl.cleanupHealthCheckRunners(ctx, logger, cluster)

		return healthCheckResult{ready: true}, nil
	}

	client, err := ctrl.kubernetesRuntime.GetClient(ctx, cluster.Metadata().ID())
	if err != nil {
		return healthCheckResult{}, err
	}

	var (
		failures []string
		interval time.Duration
	)

	// run all healthchecks so the status reflects every failing one, not just the first
	for healthcheck := range healthchecks.All() {
		outcome, err := runHealthCheck(ctx, client, healthcheck)
		if err != nil {
			return healthCheckResult{}, err
		}

		if outcome.ready {
			continue
		}

		logger.Warn("cluster healthcheck is not passing, holding the upgrade",
			zap.String("healthcheck", healthcheck.Metadata().ID()), zap.String("detail", outcome.detail))

		failure := fmt.Sprintf("%q", healthcheck.Metadata().ID())
		if outcome.detail != "" {
			failure += ": " + truncate(outcome.detail, healthCheckOutputLimit)
		}

		failures = append(failures, failure)

		// re-check at the shortest interval requested by any failing healthcheck
		checkInterval := healthcheck.TypedSpec().Value.GetInterval().AsDuration()
		if checkInterval <= 0 {
			checkInterval = defaultHealthCheckInterval
		}

		if interval == 0 || checkInterval < interval {
			interval = checkInterval
		}
	}

	if len(failures) > 0 {
		return healthCheckResult{
			reason:   "waiting for healthchecks to pass: " + strings.Join(failures, "; "),
			interval: interval,
		}, nil
	}

	// all healthchecks passed - the runner pods are no longer needed until the next batch
	ctrl.cleanupHealthCheckRunners(ctx, logger, cluster)

	return healthCheckResult{ready: true}, nil
}

// runHealthCheck ensures the healthcheck's Job exists and reports its outcome. The Job is created fresh on each
// attempt and deleted once it reaches a terminal state, so the next attempt runs a fresh check. A non-nil error
// is only returned for unexpected Kubernetes API or manifest-parsing errors.
func runHealthCheck(ctx context.Context, client *kubernetes.Client, healthcheck *omni.KubernetesHealthCheck) (healthCheckRunnerStatus, error) {
	jobName := HealthCheckRunnerNamePrefix + healthcheck.Metadata().ID()

	desiredJob, err := ComposeHealthCheckJob(jobName, healthcheck.TypedSpec().Value.GetJob())
	if err != nil {
		return healthCheckRunnerStatus{}, err
	}

	return ensureHealthCheckJob(ctx, client, desiredJob)
}

// ComposeHealthCheckJob parses the user-provided Job manifest and re-asserts the fields Omni relies on to track
// the job: a deterministic name, a cleanup label, and a config-hash annotation. The namespace comes from the
// manifest (defaulting to "default"); everything else (service account, container, command, ...) is the user's.
func ComposeHealthCheckJob(jobName, manifest string) (*batchv1.Job, error) {
	manifestJSON, err := k8syaml.ToJSON([]byte(manifest))
	if err != nil {
		return nil, fmt.Errorf("failed to convert healthcheck job manifest to JSON: %w", err)
	}

	var job batchv1.Job

	if err = json.Unmarshal(manifestJSON, &job); err != nil {
		return nil, fmt.Errorf("failed to unmarshal healthcheck job manifest: %w", err)
	}

	// Omni owns the job's identity so it can track, recreate and clean it up
	job.Name = jobName
	job.GenerateName = ""

	if job.Namespace == "" {
		job.Namespace = DefaultHealthCheckNamespace
	}

	if job.Labels == nil {
		job.Labels = map[string]string{}
	}

	job.Labels["app.kubernetes.io/name"] = HealthCheckRunnerName

	if job.Annotations == nil {
		job.Annotations = map[string]string{}
	}

	sum := sha256.Sum256([]byte(manifest))
	job.Annotations[HealthCheckConfigHashAnnotation] = hex.EncodeToString(sum[:])

	// default to capturing the tail of the container output as the termination message on failure, so Omni can
	// surface the check's stderr from the pod status (no log streaming) when it fails. The user can still
	// override this per-container.
	containers := job.Spec.Template.Spec.Containers
	for i := range containers {
		if containers[i].TerminationMessagePolicy == "" {
			containers[i].TerminationMessagePolicy = corev1.TerminationMessageFallbackToLogsOnError
		}
	}

	return &job, nil
}

// ensureHealthCheckJob drives the healthcheck job's lifecycle and reports its current outcome:
//   - the job isn't running yet -> create it and report "running";
//   - the job is still running -> report "running" (the caller re-checks on the next interval);
//   - the job failed -> report the failure reason from its status condition and re-create it to retry;
//   - the job succeeded -> delete it and report ready, so the upgrade can proceed.
//
// The job is also re-created when the manifest changes, and a job that is still being torn down is reported as
// "running" until it is gone.
func ensureHealthCheckJob(ctx context.Context, client *kubernetes.Client, desiredJob *batchv1.Job) (healthCheckRunnerStatus, error) {
	jobs := client.Clientset().BatchV1().Jobs(desiredJob.Namespace)

	job, err := jobs.Get(ctx, desiredJob.Name, v1.GetOptions{})

	switch {
	case apierrors.IsNotFound(err):
		// the job isn't running - start it
		if _, err = jobs.Create(ctx, desiredJob, v1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
			return healthCheckRunnerStatus{}, fmt.Errorf("failed to create healthcheck job: %w", err)
		}

		return healthCheckRunnerStatus{detail: healthCheckJobRunningDetail}, nil
	case err != nil:
		return healthCheckRunnerStatus{}, err
	case job.DeletionTimestamp != nil:
		// the previous job is still being torn down; wait for it to disappear before re-creating
		return healthCheckRunnerStatus{detail: healthCheckJobRunningDetail}, nil
	case job.Annotations[HealthCheckConfigHashAnnotation] != desiredJob.Annotations[HealthCheckConfigHashAnnotation]:
		// the job manifest changed - re-create the job to run the new check
		if err = deleteHealthCheckJob(ctx, client, job.Namespace, job.Name); err != nil {
			return healthCheckRunnerStatus{}, err
		}

		return healthCheckRunnerStatus{detail: healthCheckJobRunningDetail}, nil
	}

	if JobComplete(job) {
		// the check succeeded - delete the job and let the upgrade proceed
		if err = deleteHealthCheckJob(ctx, client, job.Namespace, job.Name); err != nil {
			return healthCheckRunnerStatus{}, err
		}

		return healthCheckRunnerStatus{ready: true}, nil
	}

	if condition := JobFailedCondition(job); condition != nil {
		// the check failed - prefer the container's output (captured in the pod status), falling back to the
		// job's failure condition (e.g. "BackoffLimitExceeded") when no output is available, then re-create
		// the job to retry
		detail := healthCheckJobFailureOutput(ctx, client, job)
		if detail == "" {
			detail = "healthcheck job failed"
			if condition.Reason != "" {
				detail = condition.Reason
			}

			if condition.Message != "" {
				detail += ": " + condition.Message
			}
		}

		if err = deleteHealthCheckJob(ctx, client, job.Namespace, job.Name); err != nil {
			return healthCheckRunnerStatus{}, err
		}

		return healthCheckRunnerStatus{detail: detail}, nil
	}

	// the job is still running - re-check on the next interval
	return healthCheckRunnerStatus{detail: healthCheckJobRunningDetail}, nil
}

// JobComplete reports whether the job has completed successfully.
func JobComplete(job *batchv1.Job) bool {
	for _, condition := range job.Status.Conditions {
		if condition.Type == batchv1.JobComplete {
			return condition.Status == corev1.ConditionTrue
		}
	}

	return false
}

// JobFailedCondition returns the job's JobFailed condition when it is true (carrying the failure reason and
// message), or nil when the job has not failed.
func JobFailedCondition(job *batchv1.Job) *batchv1.JobCondition {
	for i, condition := range job.Status.Conditions {
		if condition.Type == batchv1.JobFailed && condition.Status == corev1.ConditionTrue {
			return &job.Status.Conditions[i]
		}
	}

	return nil
}

// healthCheckJobFailureOutput returns the output of the failed healthcheck container, read from the job's pods.
// With terminationMessagePolicy=FallbackToLogsOnError (set by composeHealthCheckJob), the kubelet records the
// tail of the container output as the termination message, so the check's stderr is available straight from the
// pod status without streaming logs. It is best-effort: an empty string is returned when nothing is available.
func healthCheckJobFailureOutput(ctx context.Context, client *kubernetes.Client, job *batchv1.Job) string {
	pods, err := client.Clientset().CoreV1().Pods(job.Namespace).List(ctx, v1.ListOptions{
		LabelSelector: "batch.kubernetes.io/job-name=" + job.Name,
	})
	if err != nil {
		return ""
	}

	return FailedContainerMessage(pods.Items)
}

// FailedContainerMessage picks the termination message of the most recently finished failed container across the
// given pods (the latest attempt's output).
func FailedContainerMessage(pods []corev1.Pod) string {
	var (
		message string
		latest  time.Time
	)

	for i := range pods {
		for _, status := range pods[i].Status.ContainerStatuses {
			terminated := status.State.Terminated
			if terminated == nil || terminated.ExitCode == 0 {
				continue
			}

			msg := strings.TrimSpace(terminated.Message)
			if msg == "" {
				continue
			}

			if message == "" || terminated.FinishedAt.After(latest) {
				message = msg
				latest = terminated.FinishedAt.Time
			}
		}
	}

	return message
}

// cleanupHealthCheckRunners removes all healthcheck runner jobs of the cluster, so nothing is left behind once
// no healthchecks remain. It is best-effort: failing to clean up must not block the upgrade.
func (ctrl *TalosUpgradeStatusController) cleanupHealthCheckRunners(ctx context.Context, logger *zap.Logger, cluster *omni.Cluster) {
	if ctrl.kubernetesRuntime == nil {
		return
	}

	client, err := ctrl.kubernetesRuntime.GetClient(ctx, cluster.Metadata().ID())
	if err != nil {
		logger.Warn("failed to get kubernetes client to clean up healthcheck runners", zap.Error(err))

		return
	}

	deleteHealthCheckRunnerObjects(ctx, logger, client)
}

// deleteHealthCheckRunnerObjects best-effort deletes the runner jobs across all namespaces (a healthcheck job
// can pick its own namespace), selected by the runner label. All errors are swallowed (logged): cleanup must
// not block the upgrade and is retried on the next reconcile.
func deleteHealthCheckRunnerObjects(ctx context.Context, logger *zap.Logger, client *kubernetes.Client) {
	listOptions := v1.ListOptions{LabelSelector: healthCheckRunnerSelector}

	jobList, err := client.Clientset().BatchV1().Jobs(corev1.NamespaceAll).List(ctx, listOptions)
	if err != nil {
		logger.Warn("failed to list healthcheck runner jobs for cleanup", zap.Error(err))

		return
	}

	for _, job := range jobList.Items {
		if err = deleteHealthCheckJob(ctx, client, job.Namespace, job.Name); err != nil {
			logger.Warn("failed to clean up healthcheck runner job",
				zap.String("namespace", job.Namespace), zap.String("job", job.Name), zap.Error(err))
		}
	}
}

// deleteHealthCheckJob deletes the healthcheck runner job and its pods (background propagation); it is a no-op
// if the job is already gone.
func deleteHealthCheckJob(ctx context.Context, client *kubernetes.Client, namespace, jobName string) error {
	propagation := v1.DeletePropagationBackground

	if err := client.Clientset().BatchV1().Jobs(namespace).Delete(ctx, jobName, v1.DeleteOptions{
		GracePeriodSeconds: pointer.To[int64](0),
		PropagationPolicy:  &propagation,
	}); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete healthcheck job: %w", err)
	}

	return nil
}

// truncate shortens s to at most limit characters, appending an ellipsis when it was cut.
func truncate(s string, limit int) string {
	if len(s) <= limit {
		return s
	}

	return s[:limit] + "..."
}
