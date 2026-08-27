// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package cluster

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/etcdbackup/store"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/mappers"
	"github.com/siderolabs/omni/internal/backend/runtime/talos"
)

// BootstrapStatusController bootstraps clusters and keeps track of their bootstrap state.
type BootstrapStatusController struct {
	*qtransform.QController[*omni.ClusterStatus, *omni.ClusterBootstrapStatus]

	etcdBackupStoreFactory store.Factory
}

// BootstrapStatusControllerName is the name of the BootstrapStatusController.
const BootstrapStatusControllerName = "ClusterBootstrapStatusController"

const (
	// bootstrapCheckInterval is how often etcd is checked on the bootstrapped node after a bootstrap request was sent.
	bootstrapCheckInterval = 5 * time.Second

	// bootstrapRetryInterval is how long to wait before sending another bootstrap request.
	//
	// It is long on purpose: a bootstrap with a snapshot restore runs on the node long after the request
	// has returned, and a request sent while that restore is still running interrupts it and can leave
	// a broken etcd data directory behind, from which the cluster does not recover on its own. Waiting
	// too long only delays the recovery from a lost request.
	bootstrapRetryInterval = 5 * time.Minute
)

// NewBootstrapStatusController initializes BootstrapStatusController.
func NewBootstrapStatusController(etcdBackupStoreFactory store.Factory) *BootstrapStatusController {
	ctrl := &BootstrapStatusController{
		etcdBackupStoreFactory: etcdBackupStoreFactory,
	}

	ctrl.QController = qtransform.NewQController(
		qtransform.Settings[*omni.ClusterStatus, *omni.ClusterBootstrapStatus]{
			Name: BootstrapStatusControllerName,
			MapMetadataFunc: func(clusterStatus *omni.ClusterStatus) *omni.ClusterBootstrapStatus {
				return omni.NewClusterBootstrapStatus(clusterStatus.Metadata().ID())
			},
			UnmapMetadataFunc: func(bootstrapStatus *omni.ClusterBootstrapStatus) *omni.ClusterStatus {
				return omni.NewClusterStatus(bootstrapStatus.Metadata().ID())
			},
			TransformExtraOutputFunc:        ctrl.reconcile,
			FinalizerRemovalExtraOutputFunc: ctrl.finalizerRemoval,
		},
		qtransform.WithExtraMappedInput[*omni.TalosConfig](
			qtransform.MapperSameID[*omni.ClusterStatus](),
		),
		qtransform.WithExtraMappedInput[*omni.ClusterEndpoint](
			qtransform.MapperSameID[*omni.ClusterStatus](),
		),
		qtransform.WithExtraMappedInput[*omni.MachineSet](
			mappers.MapByClusterLabelOnlyControlplane[*omni.ClusterStatus](),
		),
		qtransform.WithExtraMappedInput[*omni.ClusterUUID](
			// no need to requeue anything, just allow the controller to read data (when restoring from another cluster backup)
			qtransform.MapperNone(),
		),
		qtransform.WithExtraMappedInput[*omni.BackupData](
			// no need to requeue anything, just allow the controller to read data (when restoring from another cluster backup)
			qtransform.MapperNone(),
		),
		qtransform.WithConcurrency(4),
	)

	return ctrl
}

func (ctrl *BootstrapStatusController) reconcile(
	ctx context.Context,
	r controller.ReaderWriter,
	logger *zap.Logger,
	clusterStatus *omni.ClusterStatus,
	bootstrapStatus *omni.ClusterBootstrapStatus,
) error {
	cpMachineSet, err := safe.ReaderGetByID[*omni.MachineSet](ctx, r, omni.ControlPlanesResourceID(clusterStatus.Metadata().ID()))
	if err != nil {
		if state.IsNotFoundError(err) { // missing control-plane, mark the cluster as non-bootstrapped
			bootstrapStatus.TypedSpec().Value.Bootstrapped = false
			bootstrapStatus.TypedSpec().Value.LastBootstrapAttempt = nil

			return nil
		}

		return fmt.Errorf("error getting control plane machineset for cluster '%s': %w", clusterStatus.Metadata().ID(), err)
	}

	if cpMachineSet.Metadata().Phase() == resource.PhaseTearingDown {
		bootstrapStatus.TypedSpec().Value.Bootstrapped = false
		bootstrapStatus.TypedSpec().Value.LastBootstrapAttempt = nil

		return r.RemoveFinalizer(ctx, cpMachineSet.Metadata(), BootstrapStatusControllerName)
	}

	if !cpMachineSet.Metadata().Finalizers().Has(BootstrapStatusControllerName) {
		if err = r.AddFinalizer(ctx, cpMachineSet.Metadata(), BootstrapStatusControllerName); err != nil {
			return fmt.Errorf("error adding finalizer to control plane machineset for cluster '%s': %w", clusterStatus.Metadata().ID(), err)
		}
	}

	if bootstrapStatus.TypedSpec().Value.Bootstrapped {
		return nil
	}

	if !clusterStatus.TypedSpec().Value.Available {
		return nil
	}

	if !clusterStatus.TypedSpec().Value.HasConnectedControlPlanes {
		return nil
	}

	if _, ok := clusterStatus.Metadata().Labels().Get(omni.LabelClusterTaintedByImporting); ok {
		logger.Info("cluster is being imported, therefore it's already been bootstrapped", zap.String("cluster_id", clusterStatus.Metadata().ID()))

		bootstrapStatus.TypedSpec().Value.Bootstrapped = true

		return nil
	}

	return ctrl.bootstrapCluster(ctx, r, logger, cpMachineSet, bootstrapStatus)
}

func (ctrl *BootstrapStatusController) finalizerRemoval(ctx context.Context, r controller.ReaderWriter, _ *zap.Logger, clusterStatus *omni.ClusterStatus) error {
	cpMachineSet, err := safe.ReaderGetByID[*omni.MachineSet](ctx, r, omni.ControlPlanesResourceID(clusterStatus.Metadata().ID()))
	if err != nil && !state.IsNotFoundError(err) {
		return err
	}

	if cpMachineSet != nil {
		return r.RemoveFinalizer(ctx, cpMachineSet.Metadata(), BootstrapStatusControllerName)
	}

	return nil
}

// recoverEtcdFromBackup recovers etcd of the given cluster using the given bootstrap spec.
func (ctrl *BootstrapStatusController) recoverEtcdFromBackup(
	ctx context.Context,
	r controller.Reader,
	talosCli *client.Client,
	bootstrapSpec *specs.MachineSetSpec_BootstrapSpec,
) error {
	clusterUUIDs, err := safe.ReaderListAll[*omni.ClusterUUID](ctx, r, state.WithLabelQuery(resource.LabelEqual(omni.LabelClusterUUID, bootstrapSpec.GetClusterUuid())))
	if err != nil {
		return fmt.Errorf("failed to list cluster uuids: %w", err)
	}

	if clusterUUIDs.Len() != 1 {
		return fmt.Errorf("expected exactly one cluster uuid, got %d", clusterUUIDs.Len())
	}

	clusterID := clusterUUIDs.Get(0).Metadata().ID()

	backupData, err := safe.ReaderGetByID[*omni.BackupData](ctx, r, clusterID)
	if err != nil {
		return fmt.Errorf("failed to get backup data for cluster %q: %w", clusterID, err)
	}

	backupStore, err := ctrl.etcdBackupStoreFactory.GetStore()
	if err != nil {
		return fmt.Errorf("failed to get backup store: %w", err)
	}

	downloadedBackupData, readCloser, err := backupStore.Download(ctx, backupData.TypedSpec().Value.GetEncryptionKey(), bootstrapSpec.GetClusterUuid(), bootstrapSpec.GetSnapshot())
	if err != nil {
		return fmt.Errorf("failed to download backup: %w", err)
	}

	defer readCloser.Close() //nolint:errcheck

	if downloadedBackupData.AESCBCEncryptionSecret != backupData.TypedSpec().Value.GetAesCbcEncryptionSecret() {
		return errors.New("aes cbc encryption secret mismatch")
	}

	if downloadedBackupData.SecretboxEncryptionSecret != backupData.TypedSpec().Value.GetSecretboxEncryptionSecret() {
		return errors.New("secretbox encryption secret mismatch")
	}

	if _, err = talosCli.EtcdRecover(ctx, readCloser); err != nil {
		return fmt.Errorf("failed calling talos client EtcdRecover: %w", err)
	}

	return nil
}

// bootstrapCluster sends the bootstrap request to the cluster and marks the cluster as bootstrapped once etcd is
// confirmed to be running on the node.
//
// The bootstrap API returns as soon as the etcd service is started on the node, before the bootstrap (and the
// snapshot restore, if requested) has actually happened. The node only keeps that intent in memory, so a reboot
// in that window loses it. This is why the cluster is marked as bootstrapped only once etcd is running.
func (ctrl *BootstrapStatusController) bootstrapCluster(
	ctx context.Context,
	r controller.Reader,
	logger *zap.Logger,
	cpMachineSet *omni.MachineSet,
	bootstrapStatus *omni.ClusterBootstrapStatus,
) error {
	clusterID := bootstrapStatus.Metadata().ID()

	talosCli, err := ctrl.getTalosClient(ctx, r, clusterID)
	if err != nil {
		if talos.IsClientNotReadyError(err) {
			return nil
		}

		return fmt.Errorf("error getting talos client for cluster '%s': %w", clusterID, err)
	}

	defer func() {
		if e := talosCli.Close(); e != nil {
			logger.Error("failed to close talos client", zap.Error(e))
		}
	}()

	etcdRunning, err := ctrl.isEtcdRunning(ctx, talosCli)
	if err != nil {
		return fmt.Errorf("error checking etcd on cluster '%s': %w", clusterID, err)
	}

	if etcdRunning {
		logger.Info("etcd is running, cluster is bootstrapped", zap.String("cluster_id", clusterID))

		bootstrapStatus.TypedSpec().Value.Bootstrapped = true

		return nil
	}

	// A request sent recently might still be in progress on the node. Sending another one would interrupt it, so
	// only keep checking etcd until enough time has passed to send again.
	lastAttempt := bootstrapStatus.TypedSpec().Value.GetLastBootstrapAttempt()
	if lastAttempt != nil && time.Since(lastAttempt.AsTime()) < bootstrapRetryInterval {
		return controller.NewRequeueInterval(bootstrapCheckInterval)
	}

	bootstrapSpec := cpMachineSet.TypedSpec().Value.GetBootstrapSpec()
	recoverEtcd := bootstrapSpec != nil

	if recoverEtcd {
		logger.Info(
			"recovering etcd from backup",
			zap.String("cluster_id", clusterID),
			zap.String("cluster_uuid", bootstrapSpec.GetClusterUuid()),
			zap.String("snapshot", bootstrapSpec.GetSnapshot()),
		)

		if err = ctrl.recoverEtcdFromBackup(ctx, r, talosCli, bootstrapSpec); err != nil {
			return err
		}
	}

	bootstrapStatus.TypedSpec().Value.LastBootstrapAttempt = timestamppb.Now()

	if err = talosCli.Bootstrap(ctx, &machine.BootstrapRequest{
		RecoverEtcd: recoverEtcd,
	}); err != nil {
		// The node already has etcd data, e.g. from a bootstrap that was not confirmed yet: there is nothing to
		// send, keep checking etcd instead.
		if status.Code(err) == codes.AlreadyExists {
			logger.Warn("the node has etcd data but etcd is not running, waiting for it", zap.String("cluster_id", clusterID))

			return controller.NewRequeueInterval(bootstrapCheckInterval)
		}

		// The request might have been accepted by the node even though the call failed, e.g. on a timeout. Do not
		// fail the reconcile, as that would discard the attempt time and re-send the request right away.
		logger.Error("error bootstrapping cluster", zap.String("cluster_id", clusterID), zap.Error(err))

		return controller.NewRequeueInterval(bootstrapCheckInterval)
	}

	logger.Info("bootstrapping cluster", zap.String("cluster_id", clusterID))

	return controller.NewRequeueInterval(bootstrapCheckInterval)
}

// isEtcdRunning checks whether the etcd service is running and healthy on the node.
//
// The etcd service gets there only after its preparation is done, which includes the snapshot restore
// when one was requested, so this confirms that the bootstrap has happened.
func (ctrl *BootstrapStatusController) isEtcdRunning(ctx context.Context, talosCli *client.Client) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	services, err := talosCli.ServiceInfo(ctx, "etcd")
	if err != nil {
		return false, err
	}

	for _, svc := range services {
		if svc.Service.GetState() == "Running" && svc.Service.GetHealth().GetHealthy() {
			return true, nil
		}
	}

	return false, nil
}

func (ctrl *BootstrapStatusController) getTalosClient(ctx context.Context, r controller.Reader, clusterName string) (*client.Client, error) {
	talosConfig, err := safe.ReaderGet[*omni.TalosConfig](ctx, r, omni.NewTalosConfig(clusterName).Metadata())
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, talos.NewClientNotReadyError(fmt.Errorf("talosconfig not found for cluster %q", clusterName))
		}

		return nil, fmt.Errorf("failed to get talosconfig for cluster %q: %w", clusterName, err)
	}

	clusterEndpoint, err := safe.ReaderGetByID[*omni.ClusterEndpoint](ctx, r, clusterName)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, talos.NewClientNotReadyError(fmt.Errorf("cluster endpoint not found for cluster %q", clusterName))
		}

		return nil, fmt.Errorf("failed to get cluster endpoint: %w", err)
	}

	addresses := clusterEndpoint.TypedSpec().Value.GetManagementAddresses()
	if len(addresses) == 0 {
		return nil, talos.NewClientNotReadyError(fmt.Errorf("no management addresses found for cluster %q", clusterName))
	}

	// We always pick the first management address to ensure that we always target the same node for recover & bootstrap, even across controller crashes.
	// This makes us avoid the rare case where due to some failure in GRPC calls, we could bootstrap two separate nodes.
	managementAddress := addresses[0]

	opts := talos.GetSocketOptions(managementAddress)

	if opts == nil {
		opts = append(opts, client.WithEndpoints(managementAddress))
	}

	opts = append(opts, client.WithConfig(omni.NewTalosClientConfig(talosConfig, managementAddress)))

	result, err := client.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create Talos client for cluster %q with mgmt address: %q: %w", clusterName, managementAddress, err)
	}

	return result, nil
}
