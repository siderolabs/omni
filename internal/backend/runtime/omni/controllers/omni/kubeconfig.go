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
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/controller/generic/transform"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xerrors"
	"go.uber.org/zap"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/system"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
	"github.com/siderolabs/omni/internal/pkg/certs"
)

// KubeconfigController creates omni.ClusterSecrets for each input omni.Cluster.
//
// KubeconfigController generates and stores cluster wide secrets.
type KubeconfigController = qtransform.QController[*omni.ClusterSecrets, *omni.Kubeconfig]

// NewKubeconfigController instantiates the Kubeconfig controller.
func NewKubeconfigController(certificateValidity time.Duration) *KubeconfigController {
	return qtransform.NewQController(
		qtransform.Settings[*omni.ClusterSecrets, *omni.Kubeconfig]{
			Name: "KubeconfigController",
			MapMetadataFunc: func(secrets *omni.ClusterSecrets) *omni.Kubeconfig {
				return omni.NewKubeconfig(resources.DefaultNamespace, secrets.Metadata().ID())
			},
			UnmapMetadataFunc: func(kubeconfig *omni.Kubeconfig) *omni.ClusterSecrets {
				return omni.NewClusterSecrets(resources.DefaultNamespace, kubeconfig.Metadata().ID())
			},
			TransformFunc: func(ctx context.Context, r controller.Reader, logger *zap.Logger, secrets *omni.ClusterSecrets, kubeconfig *omni.Kubeconfig) error {
				lbConfig, err := safe.ReaderGetByID[*omni.LoadBalancerConfig](ctx, r, secrets.Metadata().ID())
				if err != nil {
					if state.IsNotFoundError(err) {
						return xerrors.NewTagged[transform.SkipReconcileTag](err)
					}

					return nil
				}

				var staleCertificate bool

				if len(kubeconfig.TypedSpec().Value.Data) > 0 {
					var cfg *rest.Config

					cfg, err = clientcmd.RESTConfigFromKubeConfig(kubeconfig.TypedSpec().Value.Data)
					if err != nil {
						return fmt.Errorf("error parsing Kubernetes API config: %w", err)
					}

					staleCertificate, err = certs.IsPEMEncodedCertificateStale(cfg.CertData, certificateValidity)
					if err != nil {
						return fmt.Errorf("error checking Kubernetes API certificate: %w", err)
					}
				}

				// should always call UpdateInputsVersions to update the annotations, due to short-circuiting
				if !helpers.UpdateInputsVersions[resource.Resource](kubeconfig, secrets, lbConfig) && !staleCertificate {
					return nil
				}

				if staleCertificate {
					logger.Info("Kubernetes API certificate for cluster is stale, refreshing", zap.String("cluster", secrets.Metadata().ID()))
				}

				kubeconfig.TypedSpec().Value.Data, err = certs.GenerateKubeconfig(secrets, lbConfig, certificateValidity)
				if err != nil {
					return fmt.Errorf("error generating Kubernetes API config: %w", err)
				}

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

				return safe.Map(secrets, func(secret *omni.ClusterSecrets) (resource.Pointer, error) {
					return secret.Metadata(), nil
				})
			},
		),
		qtransform.WithExtraMappedInput(
			qtransform.MapperSameID[*omni.LoadBalancerConfig, *omni.ClusterSecrets](),
		),
	)
}
