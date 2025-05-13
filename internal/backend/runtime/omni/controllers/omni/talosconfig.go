// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"encoding/base64"
	"fmt"
	"slices"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/siderolabs/talos/pkg/machinery/role"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/system"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
	"github.com/siderolabs/omni/internal/pkg/certs"
)

// TalosConfigController creates omni.ClusterSecrets for each input omni.Cluster.
//
// TalosConfigController generates and stores cluster wide secrets.
type TalosConfigController = qtransform.QController[*omni.ClusterSecrets, *omni.TalosConfig]

// NewTalosConfigController instantiates the talosconfig controller.
func NewTalosConfigController(certificateValidity time.Duration) *TalosConfigController {
	return qtransform.NewQController(
		qtransform.Settings[*omni.ClusterSecrets, *omni.TalosConfig]{
			Name: "TalosConfigController",
			MapMetadataFunc: func(secrets *omni.ClusterSecrets) *omni.TalosConfig {
				return omni.NewTalosConfig(resources.DefaultNamespace, secrets.Metadata().ID())
			},
			UnmapMetadataFunc: func(talosConfig *omni.TalosConfig) *omni.ClusterSecrets {
				return omni.NewClusterSecrets(resources.DefaultNamespace, talosConfig.Metadata().ID())
			},
			TransformFunc: func(_ context.Context, _ controller.Reader, logger *zap.Logger, secrets *omni.ClusterSecrets, talosConfig *omni.TalosConfig) error {
				staleCertificate, err := certs.IsBase64EncodedCertificateStale(talosConfig.TypedSpec().Value.Crt, certificateValidity)
				if err != nil {
					return fmt.Errorf("error checking Talos API certificate: %w", err)
				}

				// should always call UpdateInputsVersions to update the annotations, due to short-circuiting
				if !helpers.UpdateInputsVersions(talosConfig, secrets) && !staleCertificate {
					return nil
				}

				if staleCertificate {
					logger.Info("Talos API certificate for cluster is stale, refreshing", zap.String("cluster", secrets.Metadata().ID()))
				}

				clientCert, CA, err := certs.TalosAPIClientCertificateFromSecrets(secrets, certificateValidity, role.MakeSet(role.Admin))
				if err != nil {
					return err
				}

				talosConfig.TypedSpec().Value.Ca = base64.StdEncoding.EncodeToString(CA)
				talosConfig.TypedSpec().Value.Crt = base64.StdEncoding.EncodeToString(clientCert.Crt)
				talosConfig.TypedSpec().Value.Key = base64.StdEncoding.EncodeToString(clientCert.Key)

				return nil
			},
		},
		qtransform.WithExtraMappedInput(
			func(ctx context.Context, _ *zap.Logger, r controller.QRuntime, _ *system.CertRefreshTick) ([]resource.Pointer, error) {
				// on cert refresh, queue updates for all cluster
				secrets, err := safe.ReaderListAll[*omni.ClusterSecrets](ctx, r)
				if err != nil {
					return nil, err
				}

				return slices.Collect(secrets.Pointers()), nil
			},
		),
	)
}
