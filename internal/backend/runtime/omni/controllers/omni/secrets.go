// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xerrors"
	"github.com/siderolabs/talos/pkg/machinery/config"
	talossecrets "github.com/siderolabs/talos/pkg/machinery/config/generate/secrets"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/etcdbackup"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/etcdbackup/store"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/mappers"
)

// SecretsController creates omni.ClusterSecrets for each inputed omni.Cluster.
//
// SecretsController generates and stores cluster wide secrets.
type SecretsController = qtransform.QController[*omni.Cluster, *omni.ClusterSecrets]

// NewSecretsController instantiates the secrets controller.
func NewSecretsController(etcdBackupStoreFactory store.Factory) *SecretsController {
	return qtransform.NewQController(
		qtransform.Settings[*omni.Cluster, *omni.ClusterSecrets]{
			Name: "SecretsController",
			MapMetadataFunc: func(cluster *omni.Cluster) *omni.ClusterSecrets {
				return omni.NewClusterSecrets(resources.DefaultNamespace, cluster.Metadata().ID())
			},
			UnmapMetadataFunc: func(secrets *omni.ClusterSecrets) *omni.Cluster {
				return omni.NewCluster(resources.DefaultNamespace, secrets.Metadata().ID())
			},
			TransformFunc: func(ctx context.Context, r controller.Reader, _ *zap.Logger, cluster *omni.Cluster, secrets *omni.ClusterSecrets) error {
				if len(secrets.TypedSpec().Value.GetData()) != 0 {
					// cluster already has secrets, skipping
					return nil
				}

				versionContract, err := config.ParseContractFromVersion("v" + cluster.TypedSpec().Value.TalosVersion)
				if err != nil {
					return err
				}

				cpMachineSet, err := safe.ReaderGetByID[*omni.MachineSet](ctx, r, omni.ControlPlanesResourceID(cluster.Metadata().ID()))
				if err != nil {
					if state.IsNotFoundError(err) { // need to wait for control plane to be created, so we can decide if we need to get secrets from an etcd backup or not
						return xerrors.NewTagged[qtransform.SkipReconcileTag](err)
					}

					return err
				}

				bundle, err := talossecrets.NewBundle(talossecrets.NewFixedClock(time.Now()), versionContract)
				if err != nil {
					return fmt.Errorf("error generating secrets: %w", err)
				}

				bootstrapSpec := cpMachineSet.TypedSpec().Value.GetBootstrapSpec()
				if bootstrapSpec != nil {
					var backupData etcdbackup.BackupData

					backupData, err = getBackupDataFromBootstrapSpec(ctx, r, etcdBackupStoreFactory, bootstrapSpec)
					if err != nil {
						return err
					}

					bundle.Secrets.AESCBCEncryptionSecret = backupData.AESCBCEncryptionSecret
					bundle.Secrets.SecretboxEncryptionSecret = backupData.SecretboxEncryptionSecret
				}

				data, err := json.Marshal(bundle)
				if err != nil {
					return fmt.Errorf("error marshaling secrets: %w", err)
				}

				secrets.TypedSpec().Value.Data = data

				return nil
			},
			FinalizerRemovalFunc: func(ctx context.Context, r controller.Reader, _ *zap.Logger, cluster *omni.Cluster) error {
				clusterMachines, err := r.List(
					ctx,
					resource.NewMetadata(resources.DefaultNamespace, omni.ClusterMachineType, "", resource.VersionUndefined),
					state.WithLabelQuery(
						resource.LabelEqual(omni.LabelCluster, cluster.Metadata().ID()),
					),
				)
				if err != nil {
					return err
				}

				if len(clusterMachines.Items) != 0 {
					// cluster still has machines, skipping
					return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("cluster %q still has machines", cluster.Metadata().ID())
				}

				return nil
			},
		},
		qtransform.WithExtraMappedInput(
			mappers.MapByClusterLabel[*omni.ClusterMachine, *omni.Cluster](),
		),
		qtransform.WithExtraMappedInput(
			mappers.MapByClusterLabelOnlyControlplane[*omni.MachineSet, *omni.Cluster](),
		),
		qtransform.WithExtraMappedInput(
			// no need to requeue anything, just allow the controller to read data (when restoring from another cluster backup)
			qtransform.MapperNone[*omni.ClusterUUID](),
		),
		qtransform.WithExtraMappedInput(
			// no need to requeue anything, just allow the controller to read data (when restoring from another cluster backup)
			qtransform.MapperNone[*omni.BackupData](),
		),
	)
}

func getBackupDataFromBootstrapSpec(ctx context.Context, r controller.Reader, etcdBackupStoreFactory store.Factory, spec *specs.MachineSetSpec_BootstrapSpec) (etcdbackup.BackupData, error) {
	backupStore, err := etcdBackupStoreFactory.GetStore()
	if err != nil {
		return etcdbackup.BackupData{}, fmt.Errorf("failed to get backup store: %w", err)
	}

	clusterUUIDList, err := safe.ReaderListAll[*omni.ClusterUUID](ctx, r, state.WithLabelQuery(resource.LabelEqual(omni.LabelClusterUUID, spec.GetClusterUuid())))
	if err != nil {
		return etcdbackup.BackupData{}, fmt.Errorf("failed to list cluster UUIDs for cluster %q: %w", spec.GetClusterUuid(), err)
	}

	if clusterUUIDList.Len() == 0 {
		return etcdbackup.BackupData{}, fmt.Errorf("cluster UUID %q not found", spec.GetClusterUuid())
	} else if clusterUUIDList.Len() > 1 {
		return etcdbackup.BackupData{}, fmt.Errorf("multiple cluster UUIDs found for cluster %q", spec.GetClusterUuid())
	}

	clusterUUID := clusterUUIDList.Get(0)
	clusterID := clusterUUID.Metadata().ID()

	backupData, err := safe.ReaderGetByID[*omni.BackupData](ctx, r, clusterID)
	if err != nil {
		return etcdbackup.BackupData{}, fmt.Errorf("failed to get backup data for cluster %q: %w", clusterID, err)
	}

	downloadedBackupData, readCloser, err := backupStore.Download(ctx, backupData.TypedSpec().Value.GetEncryptionKey(), spec.GetClusterUuid(), spec.GetSnapshot())
	if err != nil {
		return etcdbackup.BackupData{}, fmt.Errorf("failed to download backup data for cluster %q: %w", clusterID, err)
	}

	defer readCloser.Close() //nolint:errcheck

	return downloadedBackupData, nil
}
