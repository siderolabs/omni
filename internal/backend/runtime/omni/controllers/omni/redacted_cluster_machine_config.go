// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"errors"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/siderolabs/crypto/x509"
	"github.com/siderolabs/gen/xerrors"
	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
)

// RedactedClusterMachineConfigController manages machine configurations for each ClusterMachine.
//
// RedactedClusterMachineConfigController generates machine configuration for each created machine.
type RedactedClusterMachineConfigController = qtransform.QController[*omni.ClusterMachineConfig, *omni.RedactedClusterMachineConfig]

// NewRedactedClusterMachineConfigController initializes RedactedClusterMachineConfigController.
func NewRedactedClusterMachineConfigController() *RedactedClusterMachineConfigController {
	return qtransform.NewQController(
		qtransform.Settings[*omni.ClusterMachineConfig, *omni.RedactedClusterMachineConfig]{
			Name: "RedactedClusterMachineConfigController",
			MapMetadataFunc: func(cmc *omni.ClusterMachineConfig) *omni.RedactedClusterMachineConfig {
				return omni.NewRedactedClusterMachineConfig(resources.DefaultNamespace, cmc.Metadata().ID())
			},
			UnmapMetadataFunc: func(cmcr *omni.RedactedClusterMachineConfig) *omni.ClusterMachineConfig {
				return omni.NewClusterMachineConfig(resources.DefaultNamespace, cmcr.Metadata().ID())
			},
			TransformFunc: func(_ context.Context, _ controller.Reader, _ *zap.Logger, cmc *omni.ClusterMachineConfig, cmcr *omni.RedactedClusterMachineConfig) error {
				if !helpers.UpdateInputsVersions(cmcr, cmc) {
					return xerrors.NewTagged[qtransform.SkipReconcileTag](errors.New("config input hasn't changed"))
				}

				buffer, err := cmc.TypedSpec().Value.GetUncompressedData()
				if err != nil {
					return err
				}

				defer buffer.Free()

				data := buffer.Data()

				if data == nil {
					if err = cmcr.TypedSpec().Value.SetUncompressedData(nil); err != nil {
						return err
					}

					return nil
				}

				config, err := configloader.NewFromBytes(data)
				if err != nil {
					return err
				}

				redactedData, err := config.RedactSecrets(x509.Redacted).EncodeBytes(encoder.WithComments(encoder.CommentsDisabled))
				if err != nil {
					return err
				}

				if err = cmcr.TypedSpec().Value.SetUncompressedData(redactedData); err != nil {
					return err
				}

				helpers.CopyAllLabels(cmc, cmcr)

				return nil
			},
		},
	)
}
