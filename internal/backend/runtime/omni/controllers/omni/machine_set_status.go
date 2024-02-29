// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"net"
	"net/url"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/hashicorp/go-multierror"
	serverpb "github.com/siderolabs/discovery-api/api/v1alpha1/server/pb"
	"github.com/siderolabs/gen/pair"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime"
	"github.com/siderolabs/omni/internal/backend/runtime/kubernetes"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/configpatch"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/pkg/check"
)

type machineWithPatches = pair.Pair[*omni.ClusterMachine, *omni.ClusterMachineConfigPatches]

// MachineSetStatusController manages MachineSetStatus resource lifecycle.
//
// MachineSetStatusController creates and deletes cluster machines, handles rolling updates.
type MachineSetStatusController struct {
	discoveryClient serverpb.ClusterClient
}

// Name implements controller.Controller interface.
func (ctrl *MachineSetStatusController) Name() string {
	return "MachineSetStatusController"
}

// Inputs implements controller.Controller interface.
func (ctrl *MachineSetStatusController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: resources.DefaultNamespace,
			Type:      omni.MachineSetType,
			Kind:      controller.InputStrong,
		},
		{
			Namespace: resources.DefaultNamespace,
			Type:      omni.MachineSetNodeType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: resources.DefaultNamespace,
			Type:      omni.ClusterMachineStatusType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: resources.DefaultNamespace,
			Type:      omni.ClusterMachineType,
			Kind:      controller.InputDestroyReady,
		},
		{
			Namespace: resources.DefaultNamespace,
			Type:      omni.ControlPlaneStatusType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: resources.DefaultNamespace,
			Type:      omni.ConfigPatchType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: resources.DefaultNamespace,
			Type:      omni.ClusterMachineConfigStatusType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: resources.DefaultNamespace,
			Type:      omni.ClusterSecretsType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: resources.DefaultNamespace,
			Type:      omni.ClusterMachineIdentityType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: resources.DefaultNamespace,
			Type:      omni.LoadBalancerStatusType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: resources.DefaultNamespace,
			Type:      omni.ClusterType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: resources.DefaultNamespace,
			Type:      omni.MachineType,
			Kind:      controller.InputStrong,
		},
		{
			Namespace: resources.DefaultNamespace,
			Type:      omni.TalosConfigType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *MachineSetStatusController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: omni.MachineSetStatusType,
			Kind: controller.OutputExclusive,
		},
		{
			Type: omni.ClusterMachineType,
			Kind: controller.OutputShared,
		},
		{
			Type: omni.ClusterMachineConfigPatchesType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocognit,gocyclo,cyclop
func (ctrl *MachineSetStatusController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	conn, err := ctrl.createDiscoveryClient(ctx)
	if err != nil {
		return fmt.Errorf("error creating discovery client: %w", err)
	}

	defer func() {
		if err = conn.Close(); err != nil {
			logger.Error("error closing discovery client connection", zap.Error(err))
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		list, err := safe.ReaderListAll[*omni.MachineSet](ctx, r)
		if err != nil {
			return fmt.Errorf("error listing machine sets: %w", err)
		}

		allClusterMachines, err := safe.ReaderListAll[*omni.ClusterMachine](ctx, r)
		if err != nil {
			return fmt.Errorf("error listing all cluster machines: %w", err)
		}

		configPatchHelper, err := configpatch.NewHelper(ctx, r)
		if err != nil {
			return fmt.Errorf("error creating config patch helper: %w", err)
		}

		tracker := trackResource(r, resources.DefaultNamespace, omni.MachineSetStatusType)

		var multiErr error

		for iter := list.Iterator(); iter.Next(); {
			// process a single machine set capturing the error
			if err = func(iter safe.ListIterator[*omni.MachineSet], logger *zap.Logger) error {
				machineSet := iter.Value()
				machineSetStatus := omni.NewMachineSetStatus(resources.DefaultNamespace, machineSet.Metadata().ID())

				tracker.keep(machineSet)

				if machineSet.Metadata().Phase() != resource.PhaseTearingDown {
					if err = r.AddFinalizer(ctx, machineSet.Metadata(), ctrl.Name()); err != nil {
						return fmt.Errorf("error adding finalizer to machine set %q: %w", machineSet.Metadata().ID(), err)
					}
				}

				clusterName, ok := machineSet.Metadata().Labels().Get(omni.LabelCluster)
				if ok {
					logger = logger.With(zap.String("cluster", clusterName))
				}

				logger = logger.With(zap.String("machineset", machineSet.Metadata().ID()))

				setErr := safe.WriterModify(ctx, r, machineSetStatus, func(status *omni.MachineSetStatus) error {
					CopyLabels(machineSet, machineSetStatus, omni.LabelCluster, omni.LabelControlPlaneRole, omni.LabelWorkerRole)

					if machineSet.Metadata().Phase() == resource.PhaseTearingDown {
						status.TypedSpec().Value.Phase = specs.MachineSetPhase_Destroying
						status.TypedSpec().Value.Ready = false
						status.TypedSpec().Value.Machines = &specs.Machines{
							Total:   0,
							Healthy: 0,
						}

						machineErr := ctrl.destroyMachinesNoWait(ctx, r, logger, machineSet, allClusterMachines)
						if machineErr != nil {
							return fmt.Errorf(
								"error destroying machines (no wait), phase %q, ready %t: %w",
								status.TypedSpec().Value.GetPhase(),
								status.TypedSpec().Value.GetReady(),
								err,
							)
						}

						return nil
					}

					var machines safe.List[*omni.MachineSetNode]

					machines, err = safe.ReaderListAll[*omni.MachineSetNode](
						ctx,
						r,
						state.WithLabelQuery(resource.LabelEqual(omni.LabelMachineSet, machineSet.Metadata().ID())),
					)
					if err != nil {
						return fmt.Errorf(
							"error listing machine set nodes, phase %q, ready %t: %w",
							status.TypedSpec().Value.GetPhase(),
							status.TypedSpec().Value.GetReady(),
							err,
						)
					}

					if err = ctrl.reconcileMachines(ctx, r, logger, machines, machineSet, status, allClusterMachines, configPatchHelper); err != nil {
						return fmt.Errorf(
							"error reconciling machines, phase %q, ready %t: %w",
							status.TypedSpec().Value.GetPhase(),
							status.TypedSpec().Value.GetReady(),
							err,
						)
					}

					return nil
				})
				if setErr != nil {
					return fmt.Errorf("error modifying machine set %q: %w", machineSet.Metadata().ID(), err)
				}

				return nil
			}(iter, logger); err != nil {
				multiErr = multierror.Append(multiErr, fmt.Errorf("reconcile of machine set %q failed: %w", iter.Value().Metadata().ID(), err))
			}
		}

		if multiErr != nil {
			return multiErr
		}

		if err = tracker.cleanup(ctx); err != nil {
			return err
		}

		r.ResetRestartBackoff()
	}
}

//nolint:gocognit,gocyclo,cyclop
func (ctrl *MachineSetStatusController) updateStatus(
	ctx context.Context,
	r controller.Runtime,
	machineSetNodes safe.List[*omni.MachineSetNode],
	clusterMachines map[resource.ID]*omni.ClusterMachine,
	machineSet *omni.MachineSet,
	machineSetStatus *omni.MachineSetStatus,
) error {
	spec := machineSetStatus.TypedSpec().Value

	list, err := safe.ReaderListAll[*omni.ClusterMachineStatus](
		ctx,
		r,
		state.WithLabelQuery(resource.LabelEqual(omni.LabelMachineSet, machineSet.Metadata().ID())),
	)
	if err != nil {
		return fmt.Errorf("error listing cluster machines: %w", err)
	}

	spec.Phase = specs.MachineSetPhase_Running
	spec.Error = ""
	spec.Machines = &specs.Machines{}

	spec.Machines.Requested = uint32(machineSetNodes.Len())

	// requested machines is max(manuallyAllocatedMachines, machineClassMachineCount)
	// if machine class allocation type is not static it falls back to the actual machineSetNodes count
	// then we first compare number of machine set nodes against the number of requested machines
	// if they match we compare the number of cluster machines against the number of machine set nodes
	machineClass := machineSet.TypedSpec().Value.MachineClass
	if machineClass != nil && machineClass.AllocationType == specs.MachineSetSpec_MachineClass_Static {
		spec.Machines.Requested = machineClass.MachineCount
	}

	spec.MachineClass = machineClass

	switch {
	case machineSetNodes.Len() < int(spec.Machines.Requested):
		spec.Phase = specs.MachineSetPhase_ScalingUp
	case machineSetNodes.Len() > int(spec.Machines.Requested):
		spec.Phase = specs.MachineSetPhase_ScalingDown
	case list.Len() < machineSetNodes.Len():
		spec.Phase = specs.MachineSetPhase_ScalingUp
	case len(clusterMachines) > machineSetNodes.Len():
		spec.Phase = specs.MachineSetPhase_ScalingDown
	}

	if isControlPlane(machineSet) && machineSetNodes.Len() == 0 {
		spec.Phase = specs.MachineSetPhase_Failed
		spec.Error = "control plane machine set must have at least one node"
	}

	clusterMachineStatuses := map[resource.ID]*omni.ClusterMachineStatus{}

	for iter := list.Iterator(); iter.Next(); {
		clusterMachineStatus := iter.Value()

		clusterMachineStatuses[clusterMachineStatus.Metadata().ID()] = clusterMachineStatus

		if spec.Phase == specs.MachineSetPhase_Running {
			if clusterMachineStatus.TypedSpec().Value.GetConfigApplyStatus() == specs.ConfigApplyStatus_PENDING {
				spec.Phase = specs.MachineSetPhase_Reconfiguring
			}
		}
	}

	for _, clusterMachine := range clusterMachines {
		spec.Machines.Total++

		if clusterMachineStatus := clusterMachineStatuses[clusterMachine.Metadata().ID()]; clusterMachineStatus != nil {
			if clusterMachineStatus.TypedSpec().Value.Stage == specs.ClusterMachineStatusSpec_RUNNING && clusterMachineStatus.TypedSpec().Value.Ready {
				spec.Machines.Healthy++
			}

			if _, ok := clusterMachineStatus.Metadata().Labels().Get(omni.MachineStatusLabelConnected); ok {
				spec.Machines.Connected++
			}
		}
	}

	spec.Ready = spec.Phase == specs.MachineSetPhase_Running

	if !spec.Ready {
		return nil
	}

	configHashHasher := sha256.New()

	for iter := machineSetNodes.Iterator(); iter.Next(); {
		machineSetNode := iter.Value()

		clusterMachine := clusterMachines[machineSetNode.Metadata().ID()]
		clusterMachineStatus := clusterMachineStatuses[machineSetNode.Metadata().ID()]

		if clusterMachine == nil || clusterMachineStatus == nil {
			spec.Ready = false
			spec.Phase = specs.MachineSetPhase_ScalingUp

			return nil
		}

		clusterMachineStatusSpec := clusterMachineStatus.TypedSpec().Value

		if clusterMachineStatusSpec.Stage != specs.ClusterMachineStatusSpec_RUNNING {
			spec.Ready = false

			return nil
		}

		if !clusterMachineStatusSpec.Ready {
			spec.Ready = false

			return nil
		}

		configStatus, err := safe.ReaderGet[*omni.ClusterMachineConfigStatus](
			ctx,
			r,
			omni.NewClusterMachineConfigStatus(resources.DefaultNamespace, machineSetNode.Metadata().ID()).Metadata(),
		)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf(
				"error getting cluster machine config status for node %q: %w",
				machineSetNode.Metadata().ID(),
				err,
			)
		}

		if configStatus != nil {
			configHashHasher.Write([]byte(configStatus.TypedSpec().Value.ClusterMachineConfigSha256))
		}

		if configStatus == nil || isOutdated(clusterMachine, configStatus) {
			spec.Ready = false

			return nil
		}
	}

	// combined hash of all cluster machine config hashes
	spec.ConfigHash = hex.EncodeToString(configHashHasher.Sum(nil))

	return nil
}

func (ctrl *MachineSetStatusController) reconcileMachines(
	ctx context.Context,
	r controller.Runtime,
	logger *zap.Logger,
	expectedNodes safe.List[*omni.MachineSetNode],
	machineSet *omni.MachineSet,
	machineSetStatus *omni.MachineSetStatus,
	allClusterMachines safe.List[*omni.ClusterMachine],
	configPatchHelper *configpatch.Helper,
) error {
	clusterMachines := allClusterMachines.FilterLabelQuery(resource.LabelEqual(omni.LabelMachineSet, machineSet.Metadata().ID()))

	expectedNodesMap := map[resource.ID]*omni.MachineSetNode{}

	expectedNodes.ForEach(func(node *omni.MachineSetNode) { expectedNodesMap[node.Metadata().ID()] = node })

	clusterName, ok := machineSet.Metadata().Labels().Get(omni.LabelCluster)
	if !ok {
		return fmt.Errorf("failed to determine machine set %q cluster", machineSet.Metadata().ID())
	}

	cluster, err := safe.ReaderGet[*omni.Cluster](
		ctx,
		r,
		omni.NewCluster(resources.DefaultNamespace, clusterName).Metadata(),
	)
	if err != nil {
		if state.IsNotFoundError(err) {
			logger.Info("cluster doesn't exist: skip machines reconcile call")

			return nil
		}

		return fmt.Errorf("failed to get cluster spec %q: %w", clusterName, err)
	}

	var tearingDownMachines []*omni.ClusterMachine

	currentClusterMachines := map[resource.ID]*omni.ClusterMachine{}

	clusterMachines.ForEach(func(clusterMachine *omni.ClusterMachine) {
		currentClusterMachines[clusterMachine.Metadata().ID()] = clusterMachine

		if clusterMachine.Metadata().Phase() == resource.PhaseTearingDown {
			tearingDownMachines = append(tearingDownMachines, clusterMachine)
		}
	})

	if err = ctrl.updateStatus(ctx, r, expectedNodes, currentClusterMachines, machineSet, machineSetStatus); err != nil {
		return fmt.Errorf("failed to update machine set status: %w", err)
	}

	createMachines, err := ctrl.getMachinesToCreate(cluster, machineSet, currentClusterMachines, expectedNodes, configPatchHelper)
	if err != nil {
		return fmt.Errorf("failed to get machines to create: %w", err)
	}

	if len(createMachines) != 0 {
		err = ctrl.updateMachines(ctx, r, logger, createMachines, true)
		if err != nil {
			return fmt.Errorf("failed to create machines: %w", err)
		}

		return nil
	}

	loadbalancerInfo, err := safe.ReaderGet[*omni.LoadBalancerStatus](
		ctx,
		r,
		omni.NewLoadBalancerStatus(resources.DefaultNamespace, clusterName).Metadata(),
	)
	if err != nil {
		if state.IsNotFoundError(err) {
			logger.Info("load balancer status is unknown: skip machines update/destroy")

			return nil
		}

		return fmt.Errorf("failed to get load balancer status: %w", err)
	}

	// skip the rest if found any machine which is being torn down
	if len(tearingDownMachines) > 0 {
		err = ctrl.destroyMachines(ctx, r, logger, machineSet, tearingDownMachines)
		if err != nil {
			return fmt.Errorf("failed to destroy machines: %w", err)
		}

		return nil
	}

	updateMachines, err := ctrl.getMachinesToUpdate(ctx, r, cluster, machineSet, currentClusterMachines, expectedNodes, configPatchHelper)
	if err != nil {
		return fmt.Errorf("failed to get machines to update: %w", err)
	}

	if len(updateMachines) > 0 {
		err = ctrl.updateMachines(ctx, r, logger, updateMachines, false)
		if err != nil {
			return fmt.Errorf("failed to update machines: %w", err)
		}

		return nil
	}

	// skip the rest if Kubernetes is not up
	if !loadbalancerInfo.TypedSpec().Value.Healthy {
		logger.Info("loadbalancer is not healthy: skip destroy flow")

		return nil
	}

	destroyMachines, err := ctrl.getMachinesToDestroy(ctx, r, machineSet, clusterMachines, expectedNodesMap)
	if err != nil {
		return fmt.Errorf("failed to get machines to destroy: %w", err)
	}

	if len(destroyMachines) == 0 {
		return nil
	}

	err = ctrl.destroyMachines(ctx, r, logger, machineSet, destroyMachines)
	if err != nil {
		return fmt.Errorf("failed to destroy machines: %w", err)
	}

	return nil
}

func (ctrl *MachineSetStatusController) getMachinesToCreate(
	cluster *omni.Cluster,
	machineSet *omni.MachineSet,
	currentClusterMachines map[resource.ID]*omni.ClusterMachine,
	expectedMachines safe.List[*omni.MachineSetNode],
	configPatchHelper *configpatch.Helper,
) ([]machineWithPatches, error) {
	var clusterMachines []machineWithPatches

	for iter := expectedMachines.Iterator(); iter.Next(); {
		machineSetNode := iter.Value()

		if _, ok := currentClusterMachines[machineSetNode.Metadata().ID()]; !ok {
			clusterMachine := omni.NewClusterMachine(resources.DefaultNamespace, machineSetNode.Metadata().ID())
			clusterMachineConfigPatches := omni.NewClusterMachineConfigPatches(resources.DefaultNamespace, machineSetNode.Metadata().ID())

			item := pair.MakePair(clusterMachine, clusterMachineConfigPatches)

			CopyLabels(machineSet, clusterMachineConfigPatches, omni.LabelCluster, omni.LabelWorkerRole, omni.LabelControlPlaneRole)
			CopyLabels(machineSet, clusterMachine, omni.LabelCluster, omni.LabelWorkerRole, omni.LabelControlPlaneRole)

			clusterMachine.Metadata().Labels().Set(omni.LabelMachineSet, machineSet.Metadata().ID())
			clusterMachineConfigPatches.Metadata().Labels().Set(omni.LabelMachineSet, machineSet.Metadata().ID())

			_, err := ctrl.updateClusterMachine(cluster, item, machineSet, configPatchHelper)
			if err != nil {
				return nil, fmt.Errorf("failed to update cluster machine: %w", err)
			}

			clusterMachines = append(clusterMachines, item)
		}
	}

	return clusterMachines, nil
}

//nolint:gocognit
func (ctrl *MachineSetStatusController) getMachinesToUpdate(
	ctx context.Context,
	r controller.Runtime,
	cluster *omni.Cluster,
	machineSet *omni.MachineSet,
	currentClusterMachines map[resource.ID]*omni.ClusterMachine,
	expectedMachines safe.List[*omni.MachineSetNode],
	configPatchHelper *configpatch.Helper,
) ([]machineWithPatches, error) {
	var outdatedMachines int

	configStatuses := map[resource.ID]*omni.ClusterMachineConfigStatus{}

	if err := expectedMachines.ForEachErr(func(machineSetNode *omni.MachineSetNode) error {
		configStatus, err := safe.ReaderGet[*omni.ClusterMachineConfigStatus](
			ctx,
			r,
			omni.NewClusterMachineConfigStatus(resources.DefaultNamespace, machineSetNode.Metadata().ID()).Metadata(),
		)
		if err != nil {
			outdatedMachines++

			if state.IsNotFoundError(err) {
				return nil
			}

			return fmt.Errorf(
				"failed to get cluster machine node %q config status: %w",
				machineSetNode.Metadata().ID(),
				err,
			)
		}

		configStatuses[configStatus.Metadata().ID()] = configStatus

		clusterMachine := currentClusterMachines[configStatus.Metadata().ID()]

		if isOutdated(clusterMachine, configStatus) {
			outdatedMachines++
		}

		return nil
	}); err != nil {
		return nil, err
	}

	maxParallelism := 0
	if machineSet.TypedSpec().Value.GetUpdateStrategy() == specs.MachineSetSpec_Rolling {
		maxParallelism = max(1, int(machineSet.TypedSpec().Value.GetUpdateStrategyConfig().GetRolling().GetMaxParallelism()))
	}

	if isControlPlane(machineSet) {
		maxParallelism = 1
	}

	var items []machineWithPatches

	for iter := expectedMachines.Iterator(); iter.Next(); {
		machineSetNode := iter.Value()

		// machine is locked, skip the machine
		if _, locked := machineSetNode.Metadata().Annotations().Get(omni.MachineLocked); locked {
			continue
		}

		clusterMachine, ok := currentClusterMachines[machineSetNode.Metadata().ID()]
		if !ok {
			continue
		}

		configStatus, ok := configStatuses[clusterMachine.Metadata().ID()]

		// is the latest machine config applied to the machine?
		outdated := !ok || isOutdated(clusterMachine, configStatus)

		item := pair.MakePair(clusterMachine, omni.NewClusterMachineConfigPatches(clusterMachine.Metadata().Namespace(), clusterMachine.Metadata().ID()))

		// are there any outstanding config patches to be applied?
		updated, err := ctrl.updateClusterMachine(cluster, item, machineSet, configPatchHelper)
		if err != nil {
			return nil, fmt.Errorf("failed to update cluster machine: %w", err)
		}

		// machine config patches are up-to-date, skip the machine
		if !updated {
			continue
		}

		// the current machine needs to be updated
		switch machineSet.TypedSpec().Value.UpdateStrategy {
		case specs.MachineSetSpec_Rolling:
			// return if we have reached the max parallelism for rolling strategy
			if len(items) >= maxParallelism {
				return items, nil
			}

			// normal mode - there are no outdated machines, so simply add the current machine to the list, as it has a pending update
			if outdatedMachines == 0 {
				items = append(items, item)

				continue
			}

			// prioritization mode - there are outdated machines:
			// add the current machine (which has a pending update) to the list only if it is one of the outdated ones
			// this handles the case of a "broken" config patch which never applies
			if outdated {
				items = append(items, item)
			}
		case specs.MachineSetSpec_Unset:
			items = append(items, item)
		}
	}

	return items, nil
}

func (ctrl *MachineSetStatusController) getMachinesToDestroy(
	ctx context.Context,
	r controller.Runtime,
	machineSet *omni.MachineSet,
	currentClusterMachines safe.List[*omni.ClusterMachine],
	expectedMachines map[resource.ID]*omni.MachineSetNode,
) ([]*omni.ClusterMachine, error) {
	var clusterMachines []*omni.ClusterMachine

	machineSetSpec := machineSet.TypedSpec().Value
	maxParallelism := 0
	rolling := false

	// if the machine set has a rolling update strategy, use it
	if machineSetSpec.GetUpdateStrategy() == specs.MachineSetSpec_Rolling {
		rolling = true
		maxParallelism = max(1, int(machineSetSpec.GetDeleteStrategyConfig().GetRolling().GetMaxParallelism())) // if the max parallelism is not set (is zero), use 1
	}

	// if this is a control plane machine set, override the strategy to always use rolling strategy with max parallelism of 1
	if isControlPlane(machineSet) {
		rolling = true
		maxParallelism = 1
	}

	for iter := currentClusterMachines.Iterator(); iter.Next(); {
		clusterMachine := iter.Value()

		if rolling && len(clusterMachines) >= maxParallelism {
			break
		}

		if _, ok := expectedMachines[clusterMachine.Metadata().ID()]; !ok {
			clusterMachines = append(clusterMachines, clusterMachine)
		}
	}

	if len(clusterMachines) == 0 {
		return clusterMachines, nil
	}

	if isControlPlane(machineSet) {
		// block removing all machines for control plane machine set
		if len(clusterMachines) == currentClusterMachines.Len() {
			return nil, nil
		}

		status, err := check.EtcdStatus(ctx, r, machineSet)
		if err != nil {
			return nil, err
		}

		if err = check.CanScaleDown(status, clusterMachines[0]); err != nil {
			return nil, err
		}
	}

	return clusterMachines, nil
}

func (ctrl *MachineSetStatusController) updateMachines(
	ctx context.Context,
	r controller.Runtime,
	logger *zap.Logger,
	machinesWithPatches []machineWithPatches,
	created bool,
) error {
	action := "update"
	if created {
		action = "create"
	}

	logger.Info(fmt.Sprintf("%s machines", action), zap.Strings("machines", xslices.Map(machinesWithPatches, func(m machineWithPatches) string { return m.F1.Metadata().ID() })))

	for _, pair := range machinesWithPatches {
		clusterMachine := pair.F1

		clusterName, ok := clusterMachine.Metadata().Labels().Get(omni.LabelCluster)
		if !ok {
			return fmt.Errorf("cluster machine update doesn't have %s label", omni.LabelCluster)
		}

		if err := safe.WriterModify(ctx, r, omni.NewClusterMachineConfigPatches(resources.DefaultNamespace, clusterMachine.Metadata().ID()),
			func(res *omni.ClusterMachineConfigPatches) error {
				// update the value
				res.TypedSpec().Value.Patches = pair.F2.TypedSpec().Value.Patches

				return nil
			},
		); err != nil {
			return err
		}

		if err := safe.WriterModify(ctx, r, clusterMachine, func(res *omni.ClusterMachine) error {
			// don't update the ClusterMachine if it's still owned by another cluster
			currentClusterName, ok := res.Metadata().Labels().Get(omni.LabelCluster)
			if ok && currentClusterName != clusterName {
				return nil
			}

			// update the labels
			CopyAllLabels(clusterMachine, res)

			// update the annotations to make sure that inputResourceVersion gets updated
			CopyAllAnnotations(clusterMachine, res)

			if res.TypedSpec().Value.KubernetesVersion == "" {
				res.TypedSpec().Value.KubernetesVersion = clusterMachine.TypedSpec().Value.KubernetesVersion
			}

			return nil
		}); err != nil {
			return fmt.Errorf("error updating cluster machine %q: %w", clusterMachine.Metadata().ID(), err)
		}

		// hold the Machine via the finalizer
		if err := r.AddFinalizer(
			ctx,
			omni.NewMachine(resources.DefaultNamespace, clusterMachine.Metadata().ID()).Metadata(),
			ctrl.Name(),
		); err != nil {
			return fmt.Errorf(
				"error adding finalizer to machine %q in cluster %q: %w",
				clusterMachine.Metadata().ID(),
				clusterName,
				err,
			)
		}

		if created {
			logger.Info("added the machine to the machine set",
				zap.String("machine", clusterMachine.Metadata().ID()),
			)
		}
	}

	return nil
}

func (ctrl *MachineSetStatusController) destroyMachines(ctx context.Context, r controller.Runtime, logger *zap.Logger, machineSet *omni.MachineSet, clusterMachines []*omni.ClusterMachine) error {
	var err error

	logger.Info("destroy machines", zap.Strings("machines", xslices.Map(clusterMachines, func(m *omni.ClusterMachine) string { return m.Metadata().ID() })))

	clusterName, ok := machineSet.Metadata().Labels().Get(omni.LabelCluster)
	if !ok {
		return fmt.Errorf("failed to determine cluster name of the machine set %s", machineSet.Metadata().ID())
	}

	secrets, err := safe.ReaderGet[*omni.ClusterSecrets](ctx, r, omni.NewClusterSecrets(
		resources.DefaultNamespace,
		clusterName,
	).Metadata())
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil
		}

		return fmt.Errorf("failed to get cluster %q secrets: %w", clusterName, err)
	}

	bundle, err := omni.ToSecretsBundle(secrets)
	if err != nil {
		return fmt.Errorf("failed to convert cluster %q secrets to bundle: %w", clusterName, err)
	}

	nodeNameOccurences, clusterMachineIdentities, err := ctrl.getClusterMachineIdentities(ctx, r, clusterName)
	if err != nil {
		return err
	}

	for _, clusterMachine := range clusterMachines {
		var (
			ready bool
			err   error
		)

		clusterMachineIdentity := clusterMachineIdentities[clusterMachine.Metadata().ID()]

		if ready, err = r.Teardown(ctx, clusterMachine.Metadata()); err != nil {
			return fmt.Errorf(
				"error tearing down machine %q in cluster %q: %w",
				clusterMachine.Metadata().ID(),
				clusterName,
				err,
			)
		}

		if !ready {
			continue
		}

		if _, ok := machineSet.Metadata().Labels().Get(omni.LabelSkipTeardown); !ok && clusterMachineIdentity != nil && nodeNameOccurences[clusterMachineIdentity.TypedSpec().Value.Nodename] == 1 {
			if err = ctrl.teardownNode(ctx, clusterMachine, clusterMachineIdentity); err != nil {
				return fmt.Errorf("error tearing down node %q: %w", clusterMachineIdentity.TypedSpec().Value.Nodename, err)
			}
		}

		if _, ok := machineSet.Metadata().Labels().Get(omni.LabelSkipTeardown); !ok && clusterMachineIdentity != nil {
			if err = ctrl.deleteMember(ctx, r, bundle.Cluster.ID, clusterMachine); err != nil {
				return fmt.Errorf(
					"error deleting member %q: %w",
					clusterMachineIdentity.TypedSpec().Value.Nodename,
					err,
				)
			}
		}

		// release the Machine finalizer
		if err = r.RemoveFinalizer(
			ctx,
			omni.NewMachine(resources.DefaultNamespace, clusterMachine.Metadata().ID()).Metadata(),
			ctrl.Name(),
		); err != nil {
			return fmt.Errorf(
				"error removing finalizer from machine %q: %w",
				clusterMachine.Metadata().ID(),
				err,
			)
		}

		if err = ctrl.destroyMachine(ctx, r, clusterMachine); err != nil {
			return fmt.Errorf("error destroying machine %q: %w", clusterMachine.Metadata().ID(), err)
		}

		logger.Info("removed the machine from the machine set gracefully",
			zap.String("machine", clusterMachine.Metadata().ID()),
		)
	}

	return nil
}

func (ctrl *MachineSetStatusController) deleteMember(
	ctx context.Context,
	r controller.Runtime,
	clusterID string,
	clusterMachine *omni.ClusterMachine,
) error {
	clusterMachineIdentity, err := safe.ReaderGet[*omni.ClusterMachineIdentity](
		ctx,
		r,
		omni.NewClusterMachineIdentity(resources.DefaultNamespace, clusterMachine.Metadata().ID()).Metadata(),
	)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil
		}

		return fmt.Errorf("error getting identity: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	_, err = ctrl.discoveryClient.AffiliateDelete(ctx, &serverpb.AffiliateDeleteRequest{
		ClusterId:   clusterID,
		AffiliateId: clusterMachineIdentity.TypedSpec().Value.NodeIdentity,
	})
	if err != nil {
		return fmt.Errorf(
			"error deleting member %q: %w",
			clusterMachineIdentity.TypedSpec().Value.NodeIdentity,
			err,
		)
	}

	return nil
}

func (ctrl *MachineSetStatusController) updateClusterMachine(
	cluster *omni.Cluster,
	pair machineWithPatches,
	machineSet *omni.MachineSet,
	configPatchHelper *configpatch.Helper,
) (bool, error) {
	clusterMachine, clusterMachineConfigPatches := pair.F1, pair.F2

	patches, err := configPatchHelper.Get(clusterMachine, machineSet)
	if err != nil {
		return false, err
	}

	if !UpdateInputsVersions(clusterMachine, patches...) && clusterMachine.TypedSpec().Value.KubernetesVersion != "" {
		return false, nil
	}

	patchesRaw := make([]string, 0, len(patches))
	for _, p := range patches {
		patchesRaw = append(patchesRaw, p.TypedSpec().Value.Data)
	}

	clusterMachineConfigPatches.TypedSpec().Value.Patches = patchesRaw
	clusterMachine.TypedSpec().Value.KubernetesVersion = cluster.TypedSpec().Value.KubernetesVersion // this will only be applied once

	return true, nil
}

func (ctrl *MachineSetStatusController) destroyMachine(ctx context.Context, r controller.Runtime, clusterMachine *omni.ClusterMachine) error {
	configPatches := omni.NewClusterMachineConfigPatches(clusterMachine.Metadata().Namespace(), clusterMachine.Metadata().ID())

	if err := r.Destroy(ctx, configPatches.Metadata()); err != nil && !state.IsNotFoundError(err) {
		return err
	}

	if err := r.Destroy(ctx, clusterMachine.Metadata()); err != nil && !state.IsNotFoundError(err) {
		return err
	}

	return nil
}

// destroyMachinesNoWait kicks in when the machine is in tearing down phase.
// it removes all machines without waiting for them to be healthy, ignores upgrade strategy.
func (ctrl *MachineSetStatusController) destroyMachinesNoWait(
	ctx context.Context,
	r controller.Runtime,
	logger *zap.Logger,
	machineSet *omni.MachineSet,
	allClusterMachines safe.List[*omni.ClusterMachine],
) error {
	list := allClusterMachines.FilterLabelQuery(resource.LabelEqual(omni.LabelMachineSet, machineSet.Metadata().ID()))

	removeFinalizer := true

	for iter := list.Iterator(); iter.Next(); {
		ready, err := r.Teardown(ctx, iter.Value().Metadata())
		if err != nil {
			return fmt.Errorf("error tearing down machine %q: %w", iter.Value().Metadata().ID(), err)
		}

		if iter.Value().Metadata().Phase() == resource.PhaseRunning {
			// if it's the first time we attempt a tear down
			logger.Info("tearing down the machine from the machine set (no wait)",
				zap.String("machine", iter.Value().Metadata().ID()),
			)
		}

		if !ready {
			removeFinalizer = false

			continue
		}

		// release the Machine finalizer
		if err = r.RemoveFinalizer(
			ctx,
			omni.NewMachine(resources.DefaultNamespace, iter.Value().Metadata().ID()).Metadata(),
			ctrl.Name(),
		); err != nil {
			return fmt.Errorf("error removing finalizer from machine %q: %w", iter.Value().Metadata().ID(), err)
		}

		if err = ctrl.destroyMachine(ctx, r, iter.Value()); err != nil {
			return fmt.Errorf("error destroying machine %q: %w", iter.Value().Metadata().ID(), err)
		}

		logger.Info("removed the machine from the machine set (no wait)",
			zap.String("machine", iter.Value().Metadata().ID()),
		)
	}

	if removeFinalizer {
		err := r.RemoveFinalizer(ctx, machineSet.Metadata(), ctrl.Name())
		if err != nil {
			return fmt.Errorf("error removing finalizer from machine set %q: %w", machineSet.Metadata().ID(), err)
		}

		return nil
	}

	return nil
}

func (ctrl *MachineSetStatusController) teardownNode(
	ctx context.Context,
	clusterMachine *omni.ClusterMachine,
	clusterMachineIdentity *omni.ClusterMachineIdentity,
) error {
	clusterName, ok := clusterMachine.Metadata().Labels().Get(omni.LabelCluster)
	if !ok {
		return fmt.Errorf("cluster machine %s doesn't have cluster label set", clusterMachine.Metadata().ID())
	}

	type kubeRuntime interface {
		GetClient(ctx context.Context, cluster string) (*kubernetes.Client, error)
	}

	k8s, err := runtime.LookupInterface[kubeRuntime](kubernetes.Name)
	if err != nil {
		return err
	}

	k8sClient, err := k8s.GetClient(ctx, clusterName)
	if err != nil {
		return fmt.Errorf("error getting kubernetes client for cluster %q: %w", clusterName, err)
	}

	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	nodename := clusterMachineIdentity.TypedSpec().Value.Nodename

	err = k8sClient.Clientset().CoreV1().Nodes().Delete(ctx, nodename, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("error deleting node %q in cluster %q: %w", nodename, clusterName, err)
	}

	return nil
}

func (ctrl *MachineSetStatusController) createDiscoveryClient(ctx context.Context) (*grpc.ClientConn, error) {
	u, err := url.Parse(constants.DefaultDiscoveryServiceEndpoint)
	if err != nil {
		return nil, err
	}

	discoveryConn, err := grpc.DialContext(ctx, net.JoinHostPort(u.Host, "443"),
		grpc.WithTransportCredentials(
			credentials.NewTLS(&tls.Config{}),
		),
		grpc.WithSharedWriteBuffer(true),
	)
	if err != nil {
		return nil, err
	}

	ctrl.discoveryClient = serverpb.NewClusterClient(discoveryConn)

	return discoveryConn, nil
}

func isOutdated(clusterMachine *omni.ClusterMachine, configStatus *omni.ClusterMachineConfigStatus) bool {
	return configStatus.TypedSpec().Value.ClusterMachineVersion != clusterMachine.Metadata().Version().String() || configStatus.TypedSpec().Value.LastConfigError != ""
}

func (ctrl *MachineSetStatusController) getClusterMachineIdentities(
	ctx context.Context,
	r controller.Runtime,
	clusterName string,
) (map[string]int, map[string]*omni.ClusterMachineIdentity, error) {
	list, err := safe.ReaderListAll[*omni.ClusterMachineIdentity](
		ctx,
		r,
		state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, clusterName)),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("error listing cluster %q machine identities: %w", clusterName, err)
	}

	nodeNameOccurences := map[string]int{}
	clusterMachineIdentities := map[string]*omni.ClusterMachineIdentity{}

	list.ForEach(func(res *omni.ClusterMachineIdentity) {
		clusterMachineIdentities[res.Metadata().ID()] = res
		nodeNameOccurences[res.TypedSpec().Value.Nodename]++
	})

	return nodeNameOccurences, clusterMachineIdentities, nil
}

func isControlPlane(res resource.Resource) bool {
	_, found := res.Metadata().Labels().Get(omni.LabelControlPlaneRole)

	return found
}
