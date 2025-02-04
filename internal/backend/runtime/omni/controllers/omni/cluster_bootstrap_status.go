// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"errors"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/etcdbackup/store"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/mappers"
	"github.com/siderolabs/omni/internal/backend/runtime/talos"
)

// ClusterBootstrapStatusController manages ClusterStatus resource lifecycle.
//
// ClusterBootstrapStatusController applies the generated machine config on each corresponding machine.
type ClusterBootstrapStatusController = qtransform.QController[*omni.ClusterStatus, *omni.ClusterBootstrapStatus]

// ClusterBootstrapStatusControllerName is the name of the ClusterBootstrapStatusController.
const ClusterBootstrapStatusControllerName = "ClusterBootstrapStatusController"

// NewClusterBootstrapStatusController initializes ClusterBootstrapStatusController.
//
//nolint:gocognit
func NewClusterBootstrapStatusController(etcdBackupStoreFactory store.Factory) *ClusterBootstrapStatusController {
	return qtransform.NewQController(
		qtransform.Settings[*omni.ClusterStatus, *omni.ClusterBootstrapStatus]{
			Name: ClusterBootstrapStatusControllerName,
			MapMetadataFunc: func(clusterStatus *omni.ClusterStatus) *omni.ClusterBootstrapStatus {
				return omni.NewClusterBootstrapStatus(resources.DefaultNamespace, clusterStatus.Metadata().ID())
			},
			UnmapMetadataFunc: func(bootstrapStatus *omni.ClusterBootstrapStatus) *omni.ClusterStatus {
				return omni.NewClusterStatus(resources.DefaultNamespace, bootstrapStatus.Metadata().ID())
			},
			TransformExtraOutputFunc: func(ctx context.Context, r controller.ReaderWriter, logger *zap.Logger, clusterStatus *omni.ClusterStatus, bootstrapStatus *omni.ClusterBootstrapStatus) error {
				cpMachineSet, err := safe.ReaderGetByID[*omni.MachineSet](ctx, r, omni.ControlPlanesResourceID(clusterStatus.Metadata().ID()))
				if err != nil {
					if state.IsNotFoundError(err) { // missing control-plane, mark the cluster as non-bootstrapped
						bootstrapStatus.TypedSpec().Value.Bootstrapped = false

						return nil
					}

					return fmt.Errorf("error getting control plane machineset for cluster '%s': %w", clusterStatus.Metadata().ID(), err)
				}

				if cpMachineSet.Metadata().Phase() == resource.PhaseTearingDown {
					bootstrapStatus.TypedSpec().Value.Bootstrapped = false

					return r.RemoveFinalizer(ctx, cpMachineSet.Metadata(), ClusterBootstrapStatusControllerName)
				}

				if !cpMachineSet.Metadata().Finalizers().Has(ClusterBootstrapStatusControllerName) {
					if err = r.AddFinalizer(ctx, cpMachineSet.Metadata(), ClusterBootstrapStatusControllerName); err != nil {
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

				talosCli, err := getTalosClientForBootstrap(ctx, r, clusterStatus.Metadata().ID())
				if err != nil {
					if talos.IsClientNotReadyError(err) {
						return nil
					}

					return fmt.Errorf("error getting talos client for cluster '%s': %w", clusterStatus.Metadata().ID(), err)
				}

				defer func() {
					if e := talosCli.Close(); e != nil {
						logger.Error("failed to close talos client", zap.Error(e))
					}
				}()

				bootstrapSpec := cpMachineSet.TypedSpec().Value.GetBootstrapSpec()
				recoverEtcd := bootstrapSpec != nil

				if recoverEtcd {
					logger.Info(
						"recovering etcd from backup",
						zap.String("cluster_id", clusterStatus.Metadata().ID()),
						zap.String("cluster_uuid", bootstrapSpec.GetClusterUuid()),
						zap.String("snapshot", bootstrapSpec.GetSnapshot()),
					)

					if err = recoverEtcdFromBackup(ctx, r, talosCli, etcdBackupStoreFactory, bootstrapSpec); err != nil {
						return err
					}
				}

				if err = talosCli.Bootstrap(ctx, &machine.BootstrapRequest{
					RecoverEtcd: recoverEtcd,
				}); err != nil {
					return fmt.Errorf("error bootstrapping cluster '%s': %w", clusterStatus.Metadata().ID(), err)
				}

				logger.Info("bootstrapping cluster", zap.String("cluster_id", clusterStatus.Metadata().ID()))

				bootstrapStatus.TypedSpec().Value.Bootstrapped = true

				return nil
			},
			FinalizerRemovalExtraOutputFunc: func(ctx context.Context, r controller.ReaderWriter, _ *zap.Logger, clusterStatus *omni.ClusterStatus) error {
				cpMachineSet, err := safe.ReaderGetByID[*omni.MachineSet](ctx, r, omni.ControlPlanesResourceID(clusterStatus.Metadata().ID()))
				if err != nil && !state.IsNotFoundError(err) {
					return err
				}

				if cpMachineSet != nil {
					return r.RemoveFinalizer(ctx, cpMachineSet.Metadata(), ClusterBootstrapStatusControllerName)
				}

				return nil
			},
		},
		qtransform.WithExtraMappedInput(
			qtransform.MapperSameID[*omni.TalosConfig, *omni.ClusterStatus](),
		),
		qtransform.WithExtraMappedInput(
			qtransform.MapperSameID[*omni.ClusterEndpoint, *omni.ClusterStatus](),
		),
		qtransform.WithExtraMappedInput(
			mappers.MapByClusterLabelOnlyControlplane[*omni.MachineSet, *omni.ClusterStatus](),
		),
		qtransform.WithExtraMappedInput(
			// no need to requeue anything, just allow the controller to read data (when restoring from another cluster backup)
			qtransform.MapperNone[*omni.ClusterUUID](),
		),
		qtransform.WithExtraMappedInput(
			// no need to requeue anything, just allow the controller to read data (when restoring from another cluster backup)
			qtransform.MapperNone[*omni.BackupData](),
		),
		qtransform.WithConcurrency(4),
	)
}

// recoverEtcdFromBackup recovers etcd of the given cluster using the given bootstrap spec.
func recoverEtcdFromBackup(ctx context.Context, r controller.Reader, talosCli *client.Client,
	etcdBackupStoreFactory store.Factory, bootstrapSpec *specs.MachineSetSpec_BootstrapSpec,
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

	backupStore, err := etcdBackupStoreFactory.GetStore()
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

func getTalosClientForBootstrap(ctx context.Context, r controller.Reader, clusterName string) (*client.Client, error) {
	talosConfig, err := safe.ReaderGet[*omni.TalosConfig](ctx, r, omni.NewTalosConfig(resources.DefaultNamespace, clusterName).Metadata())
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
