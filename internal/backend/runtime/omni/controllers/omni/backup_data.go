// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xerrors"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// BackupDataController is a controller that manages BackupData.
type BackupDataController = qtransform.QController[*omni.Cluster, *omni.BackupData]

// NewBackupDataController initializes BackupDataController.
func NewBackupDataController() *BackupDataController {
	return qtransform.NewQController(
		qtransform.Settings[*omni.Cluster, *omni.BackupData]{
			Name: "BackupDataController",
			MapMetadataFunc: func(cluster *omni.Cluster) *omni.BackupData {
				return omni.NewBackupData(cluster.Metadata().ID())
			},
			UnmapMetadataFunc: func(backupData *omni.BackupData) *omni.Cluster {
				return omni.NewCluster(resources.DefaultNamespace, backupData.Metadata().ID())
			},
			TransformFunc: func(ctx context.Context, r controller.Reader, _ *zap.Logger, cluster *omni.Cluster, backupData *omni.BackupData) error {
				clusterID := cluster.Metadata().ID()

				clusterUUID, err := safe.ReaderGetByID[*omni.ClusterUUID](ctx, r, clusterID)
				if err != nil {
					if state.IsNotFoundError(err) {
						return xerrors.NewTagged[qtransform.SkipReconcileTag](err)
					}

					return fmt.Errorf("failed to get cluster %q UUID: %w", clusterID, err)
				}

				etcdBackupEncryption, err := safe.ReaderGetByID[*omni.EtcdBackupEncryption](ctx, r, clusterID)
				if err != nil {
					if state.IsNotFoundError(err) {
						return xerrors.NewTagged[qtransform.SkipReconcileTag](err)
					}

					return fmt.Errorf("failed to get cluster %q etcd backup encryption: %w", clusterID, err)
				}

				clusterSecrets, err := safe.ReaderGetByID[*omni.ClusterSecrets](ctx, r, clusterID)
				if err != nil {
					if state.IsNotFoundError(err) {
						return xerrors.NewTagged[qtransform.SkipReconcileTag](err)
					}

					return fmt.Errorf("failed to get cluster %q secrets: %w", clusterID, err)
				}

				bundle, err := omni.ToSecretsBundle(clusterSecrets)
				if err != nil {
					return fmt.Errorf("failed to decode cluster %q secrets: %w", clusterID, err)
				}

				if cluster.TypedSpec().Value.BackupConfiguration != nil && cluster.TypedSpec().Value.BackupConfiguration.Enabled {
					backupData.TypedSpec().Value.Interval = cluster.TypedSpec().Value.GetBackupConfiguration().GetInterval()
				} else {
					backupData.TypedSpec().Value.Interval = durationpb.New(0)
				}

				backupData.TypedSpec().Value.ClusterUuid = clusterUUID.TypedSpec().Value.GetUuid()
				backupData.TypedSpec().Value.EncryptionKey = etcdBackupEncryption.TypedSpec().Value.GetEncryptionKey()
				backupData.TypedSpec().Value.SecretboxEncryptionSecret = bundle.Secrets.SecretboxEncryptionSecret
				backupData.TypedSpec().Value.AesCbcEncryptionSecret = bundle.Secrets.AESCBCEncryptionSecret

				return nil
			},
		},
		qtransform.WithExtraMappedInput(
			qtransform.MapperSameID[*omni.EtcdBackupEncryption, *omni.Cluster](),
		),
		qtransform.WithExtraMappedInput(
			qtransform.MapperSameID[*omni.ClusterSecrets, *omni.Cluster](),
		),
		qtransform.WithExtraMappedInput(
			qtransform.MapperSameID[*omni.ClusterUUID, *omni.Cluster](),
		),
	)
}
