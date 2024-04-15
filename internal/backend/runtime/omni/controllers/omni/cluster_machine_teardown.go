// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"fmt"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime"
	"github.com/siderolabs/omni/internal/backend/runtime/kubernetes"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/mappers"
)

// ClusterMachineTeardownControllerName is the name of the ClusterMachineTeardownController.
const ClusterMachineTeardownControllerName = "ClusterMachineTeardownController"

// ClusterMachineTeardownController processes additional teardown steps for a machine leaving a machine set.
type ClusterMachineTeardownController struct {
	discoveryClient DiscoveryClient
	generic.NamedController
}

// DiscoveryClient is an interface for interacting with the discovery service.
type DiscoveryClient interface {
	AffiliateDelete(ctx context.Context, cluster, affiliate string) error
}

// NewClusterMachineTeardownController initializes ClusterMachineTeardownController.
func NewClusterMachineTeardownController(discoveryClient DiscoveryClient) *ClusterMachineTeardownController {
	return &ClusterMachineTeardownController{
		discoveryClient: discoveryClient,

		NamedController: generic.NamedController{
			ControllerName: ClusterMachineTeardownControllerName,
		},
	}
}

// Settings implements controller.QController interface.
func (ctrl *ClusterMachineTeardownController) Settings() controller.QSettings {
	return controller.QSettings{
		Inputs: []controller.Input{
			{
				Namespace: resources.DefaultNamespace,
				Type:      omni.ClusterMachineType,
				Kind:      controller.InputQPrimary,
			},
			{
				Namespace: resources.DefaultNamespace,
				Type:      omni.ClusterType,
				Kind:      controller.InputQMappedDestroyReady,
			},
			{
				Namespace: resources.DefaultNamespace,
				Type:      omni.ClusterSecretsType,
				Kind:      controller.InputQMapped,
			},
			{
				Namespace: resources.DefaultNamespace,
				Type:      omni.ClusterMachineIdentityType,
				Kind:      controller.InputQMapped,
			},
			// TODO: drop after adding nodes + members audit
			{
				Namespace: resources.DefaultNamespace,
				Type:      omni.MachineSetType,
				Kind:      controller.InputQMapped,
			},
		},
		Concurrency: optional.Some[uint](4),
	}
}

// MapInput implements controller.QController interface.
func (ctrl *ClusterMachineTeardownController) MapInput(ctx context.Context, logger *zap.Logger, r controller.QRuntime, ptr resource.Pointer) ([]resource.Pointer, error) {
	switch ptr.Type() {
	case omni.ClusterType:
		mapper := mappers.MapByClusterLabel[*omni.Cluster, *omni.ClusterMachine]()

		return mapper(ctx, logger, r, omni.NewCluster(ptr.Namespace(), ptr.ID()))
	case omni.ClusterMachineIdentityType:
		mapper := qtransform.MapperSameID[*omni.ClusterMachineIdentity, *omni.ClusterMachine]()

		return mapper(ctx, logger, r, omni.NewClusterMachineIdentity(ptr.Namespace(), ptr.ID()))
	case omni.ClusterSecretsType:
		mapper := mappers.MapByClusterLabel[*omni.ClusterSecrets, *omni.ClusterMachine]()

		return mapper(ctx, logger, r, omni.NewClusterSecrets(ptr.Namespace(), ptr.ID()))
	case omni.MachineSetType:
		machines, err := safe.ReaderListAll[*omni.ClusterMachine](ctx, r, state.WithLabelQuery(
			resource.LabelEqual(omni.LabelMachineSet, ptr.ID()),
		))
		if err != nil {
			return nil, err
		}

		return safe.Map(machines, func(r *omni.ClusterMachine) (resource.Pointer, error) {
			return r.Metadata(), nil
		})
	}

	return nil, fmt.Errorf("unexpected resource type %q", ptr.Type())
}

// Reconcile implements controller.QController interface.
func (ctrl *ClusterMachineTeardownController) Reconcile(ctx context.Context, logger *zap.Logger, r controller.QRuntime, ptr resource.Pointer) error {
	clusterMachine, err := safe.ReaderGetByID[*omni.ClusterMachine](ctx, r, ptr.ID())
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil
		}

		return err
	}

	clusterName, ok := clusterMachine.Metadata().Labels().Get(omni.LabelCluster)
	if !ok {
		return fmt.Errorf("failed to determine the cluster name of the cluster machine %q", ptr.ID())
	}

	logger = logger.With(zap.String("machine", clusterMachine.Metadata().ID()), zap.String("cluster", clusterName))

	cluster, err := safe.ReaderGetByID[*omni.Cluster](ctx, r, clusterName)
	if err != nil && !state.IsNotFoundError(err) {
		return err
	}

	// if the cluster is running and the machine is running add finalizer with this controller name
	if clusterMachine.Metadata().Phase() == resource.PhaseRunning {
		if !clusterMachine.Metadata().Finalizers().Has(ctrl.Name()) {
			return r.AddFinalizer(ctx, ptr, ctrl.Name())
		}

		return nil
	}

	// no finalizer on the machine skip teardown
	if !clusterMachine.Metadata().Finalizers().Has(ctrl.Name()) {
		return nil
	}

	// do not run teardown until the cluster machine config status controller does reset
	if clusterMachine.Metadata().Finalizers().Has(clusterMachineConfigControllerName) {
		logger.Info("skipping teardown, waiting for reset")

		return nil
	}

	// remove finalizer without any actions if the cluster resource is either absent or is tearing down
	if cluster == nil || cluster.Metadata().Phase() == resource.PhaseTearingDown {
		return r.RemoveFinalizer(ctx, ptr, ctrl.Name())
	}

	ctx, cancel := context.WithTimeout(ctx, time.Second*20)
	defer cancel()

	// teardown member and node
	if err = ctrl.teardownNodeMember(ctx, r, logger, clusterMachine); err != nil {
		logger.Warn("failed to teardown member or node for the cluster machine", zap.Error(err))

		// TODO: remove the rest of this "IF" when we get nodes and members audit
		machineSetName, ok := clusterMachine.Metadata().Labels().Get(omni.LabelMachineSet)
		if !ok {
			return r.RemoveFinalizer(ctx, ptr, ctrl.Name())
		}

		machineSet, e := r.Get(ctx, omni.NewMachineSet(resources.DefaultNamespace, machineSetName).Metadata())
		if e != nil {
			if state.IsNotFoundError(err) {
				return r.RemoveFinalizer(ctx, ptr, ctrl.Name())
			}

			return err
		}

		// Ignore teardown errors for CP nodes which machine set is being torn down
		if _, ok := machineSet.Metadata().Labels().Get(omni.LabelControlPlaneRole); ok && machineSet.Metadata().Phase() == resource.PhaseTearingDown {
			return r.RemoveFinalizer(ctx, ptr, ctrl.Name())
		}

		return err
	}

	logger.Info("cleaned up member and node for cluster machine")

	return r.RemoveFinalizer(ctx, ptr, ctrl.Name())
}

func (ctrl *ClusterMachineTeardownController) teardownNodeMember(
	ctx context.Context,
	r controller.QRuntime,
	logger *zap.Logger,
	clusterMachine *omni.ClusterMachine,
) error {
	clusterName, ok := clusterMachine.Metadata().Labels().Get(omni.LabelCluster)
	if !ok {
		return fmt.Errorf("failed to determine cluster name of the cluster machine %s", clusterMachine.Metadata().ID())
	}

	list, err := safe.ReaderListAll[*omni.ClusterMachineIdentity](
		ctx,
		r,
		state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, clusterName)),
	)
	if err != nil {
		return fmt.Errorf("error listing cluster %q machine identities: %w", clusterName, err)
	}

	nodeNameOccurences := map[string]int{}
	clusterMachineIdentities := map[string]*omni.ClusterMachineIdentity{}

	list.ForEach(func(res *omni.ClusterMachineIdentity) {
		clusterMachineIdentities[res.Metadata().ID()] = res
		nodeNameOccurences[res.TypedSpec().Value.Nodename]++
	})

	clusterMachineIdentity := clusterMachineIdentities[clusterMachine.Metadata().ID()]

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

	if clusterMachineIdentity == nil {
		return nil
	}

	if err = ctrl.deleteMember(ctx, r, bundle.Cluster.ID, clusterMachine, logger); err != nil {
		return err
	}

	if nodeNameOccurences[clusterMachineIdentity.TypedSpec().Value.Nodename] == 1 {
		if err = ctrl.teardownNode(ctx, clusterMachine, clusterMachineIdentity); err != nil {
			return fmt.Errorf("error tearing down node %q: %w", clusterMachineIdentity.TypedSpec().Value.Nodename, err)
		}
	}

	return nil
}

func (ctrl *ClusterMachineTeardownController) deleteMember(ctx context.Context, r controller.ReaderWriter, clusterID string, clusterMachine *omni.ClusterMachine, logger *zap.Logger) error {
	clusterMachineIdentity, err := safe.ReaderGet[*omni.ClusterMachineIdentity](ctx, r, omni.NewClusterMachineIdentity(resources.DefaultNamespace, clusterMachine.Metadata().ID()).Metadata())
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil
		}

		return fmt.Errorf("error getting identity: %w", err)
	}

	logger = logger.With(zap.String("cluster_identity", clusterID), zap.String("node_identity", clusterMachineIdentity.Metadata().ID()))

	if err = ctrl.discoveryClient.AffiliateDelete(ctx, clusterID, clusterMachineIdentity.TypedSpec().Value.NodeIdentity); err != nil {
		logger.Error("failed to delete the affiliate from the discovery service", zap.Error(err))
	} else {
		logger.Info("deleted the affiliate from the discovery service")
	}

	return nil
}

func (ctrl *ClusterMachineTeardownController) teardownNode(
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
