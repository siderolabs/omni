// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"crypto/rand"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// EtcdBackupEncryptionController manages etcd backup encryption data for each cluster.
//
// EtcdBackupEncryptionController generates etcd backup encryption key and unique UUID for each cluster.
type EtcdBackupEncryptionController = qtransform.QController[*omni.ClusterUUID, *omni.EtcdBackupEncryption]

// NewEtcdBackupEncryptionController initializes EtcdBackupEncryptionController.
func NewEtcdBackupEncryptionController() *EtcdBackupEncryptionController {
	return qtransform.NewQController(
		qtransform.Settings[*omni.ClusterUUID, *omni.EtcdBackupEncryption]{
			Name: "EtcdBackupEncryptionController",
			MapMetadataFunc: func(cluster *omni.ClusterUUID) *omni.EtcdBackupEncryption {
				return omni.NewEtcdBackupEncryption(cluster.Metadata().ID())
			},
			UnmapMetadataFunc: func(backupEncryption *omni.EtcdBackupEncryption) *omni.ClusterUUID {
				return omni.NewClusterUUID(backupEncryption.Metadata().ID())
			},
			TransformFunc: func(_ context.Context, _ controller.Reader, logger *zap.Logger, c *omni.ClusterUUID, backupEncryption *omni.EtcdBackupEncryption) error {
				value := backupEncryption.TypedSpec().Value
				if len(value.EncryptionKey) != 0 {
					return nil
				}

				key, err := generateRandomBytes()
				if err != nil {
					return fmt.Errorf("failed to generate random bytes: %w", err)
				}

				value.EncryptionKey = key

				logger.Info("generated etcd backup encryption key",
					zap.String("cluster", backupEncryption.Metadata().ID()),
					zap.String("uuid", c.TypedSpec().Value.Uuid),
				)

				return nil
			},
		},
	)
}

func generateRandomBytes() ([]byte, error) {
	key := make([]byte, 32)

	_, err := rand.Read(key)

	return key, err
}
