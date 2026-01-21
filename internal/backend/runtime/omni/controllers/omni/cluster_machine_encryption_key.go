// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"crypto/rand"
	"io"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// ClusterMachineEncryptionKeyController reflects the status of a machine that is a member of a cluster.
type ClusterMachineEncryptionKeyController = qtransform.QController[*omni.ClusterMachine, *omni.ClusterMachineEncryptionKey]

// ClusterMachineEncryptionKeyControllerName is the name of the ClusterMachineEncryptionKeyController.
const ClusterMachineEncryptionKeyControllerName = "ClusterMachineEncryptionKeyController"

// NewClusterMachineEncryptionKeyController initializes ClusterMachineEncryptionKeyController.
func NewClusterMachineEncryptionKeyController() *ClusterMachineEncryptionKeyController {
	return qtransform.NewQController(
		qtransform.Settings[*omni.ClusterMachine, *omni.ClusterMachineEncryptionKey]{
			Name: ClusterMachineEncryptionKeyControllerName,
			MapMetadataFunc: func(clusterMachine *omni.ClusterMachine) *omni.ClusterMachineEncryptionKey {
				return omni.NewClusterMachineEncryptionKey(clusterMachine.Metadata().ID())
			},
			UnmapMetadataFunc: func(clusterMachineEncryptionKey *omni.ClusterMachineEncryptionKey) *omni.ClusterMachine {
				return omni.NewClusterMachine(clusterMachineEncryptionKey.Metadata().ID())
			},
			TransformFunc: func(_ context.Context, _ controller.Reader, _ *zap.Logger, _ *omni.ClusterMachine, clusterMachineEncryptionKey *omni.ClusterMachineEncryptionKey) error {
				if clusterMachineEncryptionKey.TypedSpec().Value.Data != nil {
					return nil
				}

				key := make([]byte, 32)
				if _, err := io.ReadFull(rand.Reader, key); err != nil {
					return err
				}

				clusterMachineEncryptionKey.TypedSpec().Value.Data = key

				return nil
			},
			FinalizerRemovalFunc: func(context.Context, controller.Reader, *zap.Logger, *omni.ClusterMachine) error {
				return nil
			},
		},
		qtransform.WithIgnoreTeardownUntil(), // delete the resource only when every other controller is done with ClusterMachine
	)
}
