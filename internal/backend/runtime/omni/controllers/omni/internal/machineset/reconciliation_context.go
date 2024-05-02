// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package machineset

import (
	"context"
	"errors"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xslices"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/configpatch"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/set"
)

const (
	opDelete = iota
	opUpdate
)

// ChangeQuota defines allowed number of machine deletes and update for a single reconcile call.
type ChangeQuota struct {
	Teardown int
	Update   int
}

// Use decreases quota type by 1 if it's > 0 or returns false if no more op of the kind are available.
func (q *ChangeQuota) Use(op int) bool {
	if q == nil {
		return true
	}

	var qvalue *int

	switch op {
	case opDelete:
		qvalue = &q.Teardown
	case opUpdate:
		qvalue = &q.Update
	default:
		panic(fmt.Sprintf("unknown op kind %d", op))
	}

	if *qvalue == -1 {
		return true
	}

	if *qvalue == 0 {
		return false
	}

	*qvalue--

	return true
}

// ReconciliationContext describes all related data for one reconciliation call of the machine set status controller.
type ReconciliationContext struct {
	machineSet                      *omni.MachineSet
	cluster                         *omni.Cluster
	patchesByMachine                map[resource.ID][]*omni.ConfigPatch
	machineSetNodesMap              map[resource.ID]*omni.MachineSetNode
	clusterMachinesMap              map[resource.ID]*omni.ClusterMachine
	clusterMachineConfigStatusesMap map[resource.ID]*omni.ClusterMachineConfigStatus
	clusterMachineConfigPatchesMap  map[resource.ID]*omni.ClusterMachineConfigPatches
	clusterMachineStatusesMap       map[resource.ID]*omni.ClusterMachineStatus

	clusterName string

	runningMachineSetNodesSet set.Set[string]
	idsTearingDown            set.Set[string]
	idsUnconfigured           set.Set[string]
	idsOutdated               set.Set[string]
	idsDestroyReady           set.Set[string]

	idsToTeardown []string
	idsToCreate   []string
	idsToUpdate   []string
	idsToDestroy  []string

	lbHealthy bool
}

type patchHelper interface {
	Get(*omni.ClusterMachine, *omni.MachineSet) ([]*omni.ConfigPatch, error)
}

// BuildReconciliationContext is the COSI reader dependent method to build the reconciliation context.
func BuildReconciliationContext(
	ctx context.Context, r controller.Reader, machineSet *omni.MachineSet,
) (*ReconciliationContext, error) {
	clusterName, ok := machineSet.Metadata().Labels().Get(omni.LabelCluster)
	if !ok {
		return nil, fmt.Errorf("failed to determine the cluster of the machine set %q", machineSet.Metadata().ID())
	}

	cluster, err := safe.ReaderGetByID[*omni.Cluster](ctx, r, clusterName)
	if err != nil && !state.IsNotFoundError(err) {
		return nil, err
	}

	loadBalancerStatus, err := safe.ReaderGetByID[*omni.LoadBalancerStatus](ctx, r, clusterName)
	if err != nil && !state.IsNotFoundError(err) {
		return nil, err
	}

	query := state.WithLabelQuery(
		resource.LabelEqual(
			omni.LabelMachineSet,
			machineSet.Metadata().ID(),
		),
	)

	clusterMachines, err := safe.ReaderListAll[*omni.ClusterMachine](ctx, r, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list cluster machines for the machine set %q: %w", machineSet.Metadata().ID(), err)
	}

	machineSetNodes, err := safe.ReaderListAll[*omni.MachineSetNode](ctx, r, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list machine set nodes for the machine set %q: %w", machineSet.Metadata().ID(), err)
	}

	clusterMachineConfigStatuses, err := safe.ReaderListAll[*omni.ClusterMachineConfigStatus](ctx, r, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list cluster machine config statuses for the machine set %q: %w", machineSet.Metadata().ID(), err)
	}

	clusterMachineConfigPatches, err := safe.ReaderListAll[*omni.ClusterMachineConfigPatches](ctx, r, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list cluster machine config patches for the machine set %q: %w", machineSet.Metadata().ID(), err)
	}

	clusterMachineStatuses, err := safe.ReaderListAll[*omni.ClusterMachineStatus](ctx, r, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list cluster machine config statuses for the machine set %q: %w", machineSet.Metadata().ID(), err)
	}

	configPatchHelper, err := configpatch.NewHelper(ctx, r)
	if err != nil {
		return nil, fmt.Errorf("error creating config patch helper: %w", err)
	}

	return NewReconciliationContext(
		cluster,
		machineSet,
		loadBalancerStatus,
		configPatchHelper,
		toSlice(machineSetNodes),
		toSlice(clusterMachines),
		toSlice(clusterMachineConfigStatuses),
		toSlice(clusterMachineConfigPatches),
		toSlice(clusterMachineStatuses),
	)
}

// NewReconciliationContext creates new state for machine set status controller reconciliation flow.
func NewReconciliationContext(
	cluster *omni.Cluster,
	machineSet *omni.MachineSet,
	loadbalancerStatus *omni.LoadBalancerStatus,
	patchHelper patchHelper,
	machineSetNodes []*omni.MachineSetNode,
	clusterMachines []*omni.ClusterMachine,
	clusterMachineConfigStatuses []*omni.ClusterMachineConfigStatus,
	clusterMachineConfigPatches []*omni.ClusterMachineConfigPatches,
	clusterMachineStatuses []*omni.ClusterMachineStatus,
) (*ReconciliationContext, error) {
	clusterName, ok := machineSet.Metadata().Labels().Get(omni.LabelCluster)
	if !ok {
		return nil, fmt.Errorf("failed to determine the cluster of the machine set %q", machineSet.Metadata().ID())
	}

	rc := &ReconciliationContext{
		machineSet:       machineSet,
		cluster:          cluster,
		clusterName:      clusterName,
		patchesByMachine: map[resource.ID][]*omni.ConfigPatch{},
	}

	checkLocked := func(r *omni.MachineSetNode) bool {
		_, ok := r.Metadata().Annotations().Get(omni.MachineLocked)

		return ok
	}

	checkTearingDown := func(r *omni.ClusterMachine) bool {
		return r.Metadata().Phase() == resource.PhaseTearingDown
	}

	checkRunning := func(r *omni.MachineSetNode) bool {
		return r.Metadata().Phase() == resource.PhaseRunning
	}

	rc.clusterMachinesMap = toMap(clusterMachines)
	rc.clusterMachineConfigStatusesMap = toMap(clusterMachineConfigStatuses)
	rc.clusterMachineConfigPatchesMap = toMap(clusterMachineConfigPatches)
	rc.clusterMachineStatusesMap = toMap(clusterMachineStatuses)
	rc.machineSetNodesMap = toMap(machineSetNodes)
	rc.runningMachineSetNodesSet = toSet(xslices.Filter(machineSetNodes, checkRunning))

	clusterMachinesSet := toSet(clusterMachines)
	lockedMachinesSet := toSet(xslices.Filter(machineSetNodes, checkLocked))
	tearingDownMachinesSet := toSet(xslices.Filter(clusterMachines, checkTearingDown))
	rc.idsDestroyReady = toSet(xslices.Filter(clusterMachines, func(clusterMachine *omni.ClusterMachine) bool {
		return clusterMachine.Metadata().Phase() == resource.PhaseTearingDown && clusterMachine.Metadata().Finalizers().Empty()
	}))

	// cluster machines
	rc.idsToDestroy = set.Values(rc.idsDestroyReady)

	// if tearing down then all machines need to be torn down
	if machineSet.Metadata().Phase() == resource.PhaseTearingDown {
		rc.idsToTeardown = set.Values(
			set.Difference(
				clusterMachinesSet,
				tearingDownMachinesSet,
			),
		)

		return rc, nil
	}

	rc.idsToTeardown = set.Values(
		set.Difference(
			clusterMachinesSet,
			tearingDownMachinesSet,
			rc.runningMachineSetNodesSet,
			lockedMachinesSet,
		),
	)

	rc.idsToCreate = set.Values(set.Difference(rc.runningMachineSetNodesSet, clusterMachinesSet))

	rc.idsTearingDown = set.Difference(tearingDownMachinesSet, rc.idsDestroyReady)

	updateCandidates := set.Values(
		set.Difference(
			set.Intersection(
				rc.runningMachineSetNodesSet,
				clusterMachinesSet,
			),
			tearingDownMachinesSet,
			lockedMachinesSet,
		),
	)

	for id := range set.Union(rc.runningMachineSetNodesSet, clusterMachinesSet) {
		clusterMachine := omni.NewClusterMachine(resources.DefaultNamespace, id)

		helpers.CopyAllLabels(machineSet, clusterMachine)

		clusterMachine.Metadata().Labels().Set(omni.LabelMachineSet, machineSet.Metadata().ID())

		patches, err := patchHelper.Get(clusterMachine, machineSet)
		if err != nil {
			return nil, err
		}

		rc.patchesByMachine[clusterMachine.Metadata().ID()] = patches
	}

	for _, id := range updateCandidates {
		clusterMachine := rc.clusterMachinesMap[id].DeepCopy().(*omni.ClusterMachine) //nolint:forcetypeassert,errcheck

		_, ok := rc.clusterMachineConfigPatchesMap[id]
		if !ok {
			rc.idsToUpdate = append(rc.idsToUpdate, id)

			continue
		}

		patches := rc.patchesByMachine[id]

		if helpers.UpdateInputsVersions(clusterMachine, patches...) {
			rc.idsToUpdate = append(rc.idsToUpdate, id)
		}
	}

	rc.idsOutdated = make(set.Set[string])
	rc.idsUnconfigured = make(set.Set[string])

	for id := range set.Difference(clusterMachinesSet, tearingDownMachinesSet) {
		clusterMachineConfigStatus, ok := rc.clusterMachineConfigStatusesMap[id]
		if !ok {
			rc.idsUnconfigured.Add(id)

			continue
		}

		if isUpdating(rc.clusterMachinesMap[id], clusterMachineConfigStatus) {
			rc.idsOutdated.Add(id)
		}
	}

	if loadbalancerStatus != nil {
		rc.lbHealthy = loadbalancerStatus.TypedSpec().Value.Healthy
	}

	return rc, nil
}

// GetMachinesToTeardown returns all machine IDs which have ClusterMachine but no MachineSetNode.
func (rc *ReconciliationContext) GetMachinesToTeardown() []string {
	return rc.idsToTeardown
}

// GetMachinesToDestroy returns all machines ready to be destroyed.
func (rc *ReconciliationContext) GetMachinesToDestroy() []string {
	return rc.idsToDestroy
}

// GetMachinesToCreate returns all machine IDs which have MachineSetNode but no ClusterMachine.
func (rc *ReconciliationContext) GetMachinesToCreate() []string {
	return rc.idsToCreate
}

// GetMachinesToUpdate returns all machine IDs which have outdated config patches.
func (rc *ReconciliationContext) GetMachinesToUpdate() []string {
	return rc.idsToUpdate
}

// GetTearingDownMachines returns all ClusterMachines in TearingDown phase.
func (rc *ReconciliationContext) GetTearingDownMachines() set.Set[string] {
	return rc.idsTearingDown
}

// GetUpdatingMachines returns all ClusterMachines which have outdated config patches, or not configured at all.
func (rc *ReconciliationContext) GetUpdatingMachines() set.Set[string] {
	return set.Union(
		rc.idsOutdated,
		rc.idsUnconfigured,
	)
}

// GetOutdatedMachines returns the list of machines which are currently being configured.
func (rc *ReconciliationContext) GetOutdatedMachines() set.Set[string] {
	return rc.idsOutdated
}

// GetKubernetesVersion reads kubernetes version from the related cluster if the cluster exists.
func (rc *ReconciliationContext) GetKubernetesVersion() (string, error) {
	if rc.cluster == nil {
		return "", errors.New("failed to get kubernetes version for the machine set as the cluster couldn't be found")
	}

	return rc.cluster.TypedSpec().Value.KubernetesVersion, nil
}

// GetClusterName reads cluster name from the context.
func (rc *ReconciliationContext) GetClusterName() string {
	return rc.clusterName
}

// GetMachineSet reads the related machine set resource.
func (rc *ReconciliationContext) GetMachineSet() *omni.MachineSet {
	return rc.machineSet
}

// GetMachineSetNodes reads the related machine set nodes resources.
func (rc *ReconciliationContext) GetMachineSetNodes() map[resource.ID]*omni.MachineSetNode {
	return rc.machineSetNodesMap
}

// GetClusterMachines reads the related machine set resources.
func (rc *ReconciliationContext) GetClusterMachines() map[resource.ID]*omni.ClusterMachine {
	return rc.clusterMachinesMap
}

// GetRunningClusterMachines gets all cluster machines except destroy ready ones.
func (rc *ReconciliationContext) GetRunningClusterMachines() map[resource.ID]*omni.ClusterMachine {
	machines := make(map[resource.ID]*omni.ClusterMachine, len(rc.clusterMachinesMap)-len(rc.idsToDestroy))

	for id, cm := range rc.clusterMachinesMap {
		if rc.idsDestroyReady.Contains(id) {
			continue
		}

		machines[id] = cm
	}

	return machines
}

// GetRunningMachineSetNodes gets all machine set nodes in running phase.
func (rc *ReconciliationContext) GetRunningMachineSetNodes() map[resource.ID]*omni.MachineSetNode {
	machines := make(map[resource.ID]*omni.MachineSetNode, len(rc.runningMachineSetNodesSet))

	for id, cm := range rc.machineSetNodesMap {
		if !rc.runningMachineSetNodesSet.Contains(id) {
			continue
		}

		machines[id] = cm
	}

	return machines
}

// GetClusterMachineStatuses reads the related machine set resources.
func (rc *ReconciliationContext) GetClusterMachineStatuses() map[resource.ID]*omni.ClusterMachineStatus {
	return rc.clusterMachineStatusesMap
}

// GetClusterMachineConfigStatuses reads the related machine set resources.
func (rc *ReconciliationContext) GetClusterMachineConfigStatuses() map[resource.ID]*omni.ClusterMachineConfigStatus {
	return rc.clusterMachineConfigStatusesMap
}

// GetClusterMachine by the id.
func (rc *ReconciliationContext) GetClusterMachine(id resource.ID) (*omni.ClusterMachine, bool) {
	cm, ok := rc.clusterMachinesMap[id]

	return cm, ok
}

// GetClusterMachineConfigStatus by the id.
func (rc *ReconciliationContext) GetClusterMachineConfigStatus(id resource.ID) (*omni.ClusterMachineConfigStatus, bool) {
	cm, ok := rc.clusterMachineConfigStatusesMap[id]

	return cm, ok
}

// GetConfigPatches reads previosly collected confignpatches for a machine.
func (rc *ReconciliationContext) GetConfigPatches(id resource.ID) []*omni.ConfigPatch {
	return rc.patchesByMachine[id]
}

// LBHealthy returns the health status of the loadbalancer for the current cluster.
func (rc *ReconciliationContext) LBHealthy() bool {
	return rc.lbHealthy
}

// CalculateQuota computes limits for scale down and update basing on the machine set max update parallelism and machine set role.
func (rc *ReconciliationContext) CalculateQuota() ChangeQuota {
	var (
		quota          ChangeQuota
		machineSetSpec = rc.machineSet.TypedSpec().Value
	)

	quota.Teardown = getParallelismOrDefault(machineSetSpec.DeleteStrategy, machineSetSpec.DeleteStrategyConfig, -1)
	quota.Update = getParallelismOrDefault(machineSetSpec.UpdateStrategy, machineSetSpec.UpdateStrategyConfig, 1)

	// final delete quota is MaxParallelism minus machines in tearing down phase
	if quota.Teardown > 0 {
		quota.Teardown -= len(rc.idsTearingDown)

		if quota.Teardown < 0 {
			quota.Teardown = 0
		}
	}

	// final update quota is MaxParallelism minus currently updated machines count
	if quota.Update > 0 {
		quota.Update -= len(rc.GetUpdatingMachines())

		if quota.Update < 0 {
			quota.Update = 0
		}
	}

	return quota
}

func getParallelismOrDefault(strategyType specs.MachineSetSpec_UpdateStrategy, strategy *specs.MachineSetSpec_UpdateStrategyConfig, def int) int {
	if strategyType == specs.MachineSetSpec_Rolling {
		if strategy == nil {
			return def
		}

		return int(strategy.Rolling.MaxParallelism)
	}

	return def
}

func isUpdating(clusterMachine *omni.ClusterMachine, clusterMachineConfigStatus *omni.ClusterMachineConfigStatus) bool {
	return clusterMachineConfigStatus.TypedSpec().Value.ClusterMachineVersion != clusterMachine.Metadata().Version().String() || clusterMachineConfigStatus.TypedSpec().Value.LastConfigError != ""
}

func toSet[T resource.Resource](resources []T) set.Set[resource.ID] {
	return set.Set[resource.ID](xslices.ToSetFunc(resources, func(r T) resource.ID {
		return r.Metadata().ID()
	}))
}

func toMap[T resource.Resource](resources []T) map[resource.ID]T {
	return xslices.ToMap(resources, func(r T) (resource.ID, T) {
		return r.Metadata().ID(), r
	})
}
