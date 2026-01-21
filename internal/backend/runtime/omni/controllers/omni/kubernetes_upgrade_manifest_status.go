// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/go-kubernetes/kubernetes/manifests"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/rest"

	"github.com/siderolabs/omni/client/api/common"
	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/panichandler"
	"github.com/siderolabs/omni/internal/backend/runtime"
	"github.com/siderolabs/omni/internal/backend/runtime/kubernetes"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/mappers"
	"github.com/siderolabs/omni/internal/backend/runtime/talos"
)

// KubernetesUpgradeManifestStatusController keeps information about bootstrap manifests being out of sync.
type KubernetesUpgradeManifestStatusController = qtransform.QController[*omni.ClusterSecrets, *omni.KubernetesUpgradeManifestStatus]

// KubernetesUpgradeManifestStatusControllerName is the name of KubernetesUpgradeManifestStatusController.
const KubernetesUpgradeManifestStatusControllerName = "KubernetesUpgradeManifestStatusController"

// ManifestDryRunSyncTimeout is the timeout for dry run syncing manifests.
const ManifestDryRunSyncTimeout = 5 * time.Minute

// NewKubernetesUpgradeManifestStatusController initializes KubernetesUpgradeManifestStatusController.
//
//nolint:gocognit,cyclop,gocyclo,maintidx
func NewKubernetesUpgradeManifestStatusController() *KubernetesUpgradeManifestStatusController {
	return qtransform.NewQController(
		qtransform.Settings[*omni.ClusterSecrets, *omni.KubernetesUpgradeManifestStatus]{
			Name: KubernetesUpgradeManifestStatusControllerName,
			MapMetadataFunc: func(upgradeStatus *omni.ClusterSecrets) *omni.KubernetesUpgradeManifestStatus {
				return omni.NewKubernetesUpgradeManifestStatus(upgradeStatus.Metadata().ID())
			},
			UnmapMetadataFunc: func(manifestStatus *omni.KubernetesUpgradeManifestStatus) *omni.ClusterSecrets {
				return omni.NewClusterSecrets(manifestStatus.Metadata().ID())
			},
			TransformFunc: func(ctx context.Context, r controller.Reader, logger *zap.Logger, clusterSecrets *omni.ClusterSecrets, manifestStatus *omni.KubernetesUpgradeManifestStatus) error {
				clusterID := clusterSecrets.Metadata().ID()

				loadbalancerStatus, err := safe.ReaderGet[*omni.LoadBalancerStatus](ctx, r, omni.NewLoadBalancerStatus(clusterID).Metadata())
				if err != nil && !state.IsNotFoundError(err) {
					return fmt.Errorf("failed to get loadbalancer status: %w", err)
				}

				// lb not ready
				if loadbalancerStatus == nil || !loadbalancerStatus.TypedSpec().Value.Healthy {
					return nil
				}

				clusterStatus, err := safe.ReaderGet[*omni.ClusterStatus](ctx, r, omni.NewClusterStatus(clusterID).Metadata())
				if err != nil && !state.IsNotFoundError(err) {
					return fmt.Errorf("failed to get cluster status: %w", err)
				}

				if clusterStatus == nil {
					return nil
				}

				if !clusterStatus.TypedSpec().Value.Ready || clusterStatus.TypedSpec().Value.Phase != specs.ClusterStatusSpec_RUNNING {
					return nil
				}

				if !clusterStatus.TypedSpec().Value.HasConnectedControlPlanes {
					return nil
				}

				k8sUpgradeStatus, err := safe.ReaderGet[*omni.KubernetesUpgradeStatus](ctx, r, omni.NewKubernetesUpgradeStatus(clusterID).Metadata())
				if err != nil && !state.IsNotFoundError(err) {
					return fmt.Errorf("failed to get kubernetes upgrade status: %w", err)
				}

				// k8s upgrade not ready
				if k8sUpgradeStatus == nil || k8sUpgradeStatus.TypedSpec().Value.Phase != specs.KubernetesUpgradeStatusSpec_Done {
					return nil
				}

				talosUpgradeStatus, err := safe.ReaderGet[*omni.TalosUpgradeStatus](ctx, r, omni.NewTalosUpgradeStatus(clusterID).Metadata())
				if err != nil && !state.IsNotFoundError(err) {
					return fmt.Errorf("failed to get talos upgrade status: %w", err)
				}

				// talos upgrade not ready
				if talosUpgradeStatus == nil || talosUpgradeStatus.TypedSpec().Value.Phase != specs.TalosUpgradeStatusSpec_Done {
					return nil
				}

				// get the controlplane machine set status
				machineSetStatuses, err := safe.ReaderList[*omni.MachineSetStatus](ctx, r, omni.NewMachineSetStatus("").Metadata(),
					state.WithLabelQuery(
						resource.LabelEqual(omni.LabelCluster, clusterID),
						resource.LabelExists(omni.LabelControlPlaneRole),
					))
				if err != nil {
					return fmt.Errorf("failed to get machine set status: %w", err)
				}

				if machineSetStatuses.Len() != 1 {
					// unexpected, but log an error skip it
					logger.Error("unexpected number of controlplane machine sets", zap.String("cluster", clusterID), zap.Int("count", machineSetStatuses.Len()))

					return nil
				}

				controlplaneMachineSetStatus := machineSetStatuses.Get(0)

				// skip checking bootstrap manifests if none of the following changed:
				// - kubernetes upgrade status (Kubernetes version)
				// - talos upgrade status (Talos version)
				// - controlplane machine set aggregated config hash (controlplane ConfigPatches)
				if !helpers.UpdateInputsAnnotation(
					manifestStatus,
					k8sUpgradeStatus.Metadata().Version().String(),
					talosUpgradeStatus.Metadata().Version().String(),
					controlplaneMachineSetStatus.TypedSpec().Value.ConfigHash,
				) {
					logger.Debug("skipping bootstrap manifests check", zap.String("cluster", clusterID))

					return nil
				}

				// we are ready to perform manifest checks, set a global timeout for the operation
				ctx, err = r.ContextWithTeardown(ctx, omni.NewMachineSet(controlplaneMachineSetStatus.Metadata().ID()).Metadata())
				if err != nil {
					return fmt.Errorf("failed to set teardown context: %w", err)
				}

				ctx, cancel := context.WithTimeout(ctx, ManifestDryRunSyncTimeout)
				defer cancel()

				logger.Info("performing bootstrap manifests check", zap.String("cluster", clusterID))

				// now we are ready to perform manifest checks
				type talosClientProvider interface {
					GetClient(ctx context.Context, clusterName string) (*talos.Client, error)
				}

				talosRuntime, err := runtime.LookupInterface[talosClientProvider](talos.Name)
				if err != nil {
					return err
				}

				talosClient, err := talosRuntime.GetClient(ctx, clusterID)
				if err != nil {
					return fmt.Errorf("failed to get talos client: %w", err)
				}

				manifestStatus.TypedSpec().Value.OutOfSync = 0
				manifestStatus.TypedSpec().Value.LastFatalError = ""

				bootstrapManifests, err := manifests.GetBootstrapManifests(ctx, talosClient.COSI, nil)
				if err != nil {
					switch client.StatusCode(err) { //nolint:exhaustive
					case codes.ResourceExhausted:
						// bootstrap manifests are too large, log, but don't fail the controller
						logger.Error("failed to get bootstrap manifests", zap.String("cluster", clusterID), zap.Error(err))
						manifestStatus.TypedSpec().Value.LastFatalError = err.Error()

						return nil
					case codes.Unavailable:
						if strings.Contains(err.Error(), "x509: certificate has expired or is not yet valid") {
							// time is out of sync badly on the machine, no reason to keep trying
							logger.Error("failed to get bootstrap manifests", zap.String("cluster", clusterID), zap.Error(err))
							manifestStatus.TypedSpec().Value.LastFatalError = err.Error()

							return nil
						}
					}

					return fmt.Errorf("failed to get manifests: %w", err)
				}

				type kubernetesConfigurator interface {
					GetKubeconfig(ctx context.Context, context *common.Context) (*rest.Config, error)
				}

				kubernetesRuntime, err := runtime.LookupInterface[kubernetesConfigurator](kubernetes.Name)
				if err != nil {
					return err
				}

				cfg, err := kubernetesRuntime.GetKubeconfig(ctx, &common.Context{Name: clusterID})
				if err != nil {
					return fmt.Errorf("failed to get kubeconfig: %w", err)
				}

				errCh := make(chan error, 1)
				resultCh := make(chan manifests.SyncResult)

				panichandler.Go(func() {
					errCh <- manifests.Sync(ctx, bootstrapManifests, cfg, true, resultCh)
				}, logger)

				for {
					select {
					case err := <-errCh:
						if err != nil {
							if apierrors.IsInvalid(err) || apierrors.IsBadRequest(err) || apierrors.IsForbidden(err) || apierrors.IsRequestEntityTooLargeError(err) || webhookError(err) {
								// bootstrap manifests are invalid, log, but don't fail the controller
								logger.Error("failed to sync bootstrap manifests", zap.String("cluster", clusterID), zap.Error(err))
								manifestStatus.TypedSpec().Value.LastFatalError = err.Error()

								return nil
							}

							return fmt.Errorf("failed to dry run sync manifests: %w", err)
						}

						return nil
					case result := <-resultCh:
						if !result.Skipped {
							manifestStatus.TypedSpec().Value.OutOfSync++
						}
					}
				}
			},
		},
		qtransform.WithExtraMappedInput[*omni.LoadBalancerStatus](
			qtransform.MapperSameID[*omni.ClusterSecrets](),
		),
		qtransform.WithExtraMappedInput[*omni.ClusterStatus](
			qtransform.MapperSameID[*omni.ClusterSecrets](),
		),
		qtransform.WithExtraMappedInput[*omni.KubernetesUpgradeStatus](
			qtransform.MapperSameID[*omni.ClusterSecrets](),
		),
		qtransform.WithExtraMappedInput[*omni.TalosUpgradeStatus](
			qtransform.MapperSameID[*omni.ClusterSecrets](),
		),
		qtransform.WithExtraMappedInput[*omni.MachineSetStatus](
			mappers.MapByClusterLabelOnlyControlplane[*omni.ClusterSecrets](),
		),
		qtransform.WithExtraMappedInput[*omni.MachineSet](
			qtransform.MapperNone(),
		),
		qtransform.WithConcurrency(4),
	)
}

func webhookError(err error) bool {
	msg := err.Error()

	return strings.Contains(msg, "failed calling webhook") || strings.Contains(msg, "failed to call webhook")
}
