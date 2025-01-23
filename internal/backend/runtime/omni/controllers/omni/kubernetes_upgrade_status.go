// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"fmt"
	"reflect"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xerrors"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/kubernetes"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/mappers"
)

// KubernetesUpgradeStatusController manages KubernetesUpgradeStatus performing a Kubernetes upgrade.
//
// KubernetesUpgradeStatusController upgrades Kubernetes component versions in the cluster.
type KubernetesUpgradeStatusController = qtransform.QController[*omni.Cluster, *omni.KubernetesUpgradeStatus]

// KubernetesUpgradeStatusControllerName is the name of KubernetesUpgradeStatusController.
const KubernetesUpgradeStatusControllerName = "KubernetesUpgradeStatusController"

// NewKubernetesUpgradeStatusController initializes KubernetesUpgradeStatusController.
//
//nolint:gocognit,cyclop,gocyclo,maintidx
func NewKubernetesUpgradeStatusController() *KubernetesUpgradeStatusController {
	return qtransform.NewQController(
		qtransform.Settings[*omni.Cluster, *omni.KubernetesUpgradeStatus]{
			Name: KubernetesUpgradeStatusControllerName,
			MapMetadataFunc: func(cluster *omni.Cluster) *omni.KubernetesUpgradeStatus {
				return omni.NewKubernetesUpgradeStatus(resources.DefaultNamespace, cluster.Metadata().ID())
			},
			UnmapMetadataFunc: func(upgradeStatus *omni.KubernetesUpgradeStatus) *omni.Cluster {
				return omni.NewCluster(resources.DefaultNamespace, upgradeStatus.Metadata().ID())
			},
			TransformExtraOutputFunc: func(ctx context.Context, r controller.ReaderWriter, _ *zap.Logger, cluster *omni.Cluster, upgradeStatus *omni.KubernetesUpgradeStatus) error {
				kubernetesStatus, err := safe.ReaderGet[*omni.KubernetesStatus](ctx, r, omni.NewKubernetesStatus(resources.DefaultNamespace, cluster.Metadata().ID()).Metadata())
				if err != nil {
					if state.IsNotFoundError(err) {
						return xerrors.NewTagged[qtransform.SkipReconcileTag](err)
					}

					return err
				}

				// if not reverting to the previous successful version, perform pre-checks on each step
				versionMismatch := cluster.TypedSpec().Value.KubernetesVersion != upgradeStatus.TypedSpec().Value.LastUpgradeVersion

				if versionMismatch {
					// set the status to 'upgrading' if there is a version mismatch
					upgradeStatus.TypedSpec().Value.Phase = specs.KubernetesUpgradeStatusSpec_Upgrading
					upgradeStatus.TypedSpec().Value.CurrentUpgradeVersion = cluster.TypedSpec().Value.KubernetesVersion
				} else {
					upgradeStatus.TypedSpec().Value.CurrentUpgradeVersion = ""
				}

				clusterStatus, err := safe.ReaderGet[*omni.ClusterStatus](ctx, r, omni.NewClusterStatus(resources.DefaultNamespace, cluster.Metadata().ID()).Metadata())
				if err != nil {
					if state.IsNotFoundError(err) {
						return xerrors.NewTagged[qtransform.SkipReconcileTag](err)
					}

					return err
				}

				if clusterStatus.TypedSpec().Value.Phase != specs.ClusterStatusSpec_RUNNING || !clusterStatus.TypedSpec().Value.Ready {
					// if the cluster is not ready, and doing "normal" upgrade, set the status to 'waiting for the cluster to be ready'
					if versionMismatch {
						if upgradeStatus.TypedSpec().Value.Phase == specs.KubernetesUpgradeStatusSpec_Upgrading {
							upgradeStatus.TypedSpec().Value.Status = "waiting for the cluster to be ready"
						}

						return nil
					}

					// if performing a revert while a cluster is not ready, force all components to be not ready, as Kubernetes status might not be correctly gathered
					for _, node := range kubernetesStatus.TypedSpec().Value.Nodes {
						node.Ready = false
					}

					for _, nodePods := range kubernetesStatus.TypedSpec().Value.StaticPods {
						for _, pod := range nodePods.StaticPods {
							pod.Ready = false
						}
					}
				}

				// build a mapping of nodenames to machine IDs
				nodenameToMachineMap, err := kubernetes.NewMachineMap(ctx, r, cluster)
				if err != nil {
					return fmt.Errorf("failed to build nodename to machine map: %w", err)
				}

				// check if all Kubernetes components are ready
				upgradePath := kubernetes.CalculateUpgradePath(nodenameToMachineMap, kubernetesStatus, cluster.TypedSpec().Value.KubernetesVersion)

				upgradeStatus.Metadata().Labels().Set(omni.LabelCluster, cluster.Metadata().ID())

				switch {
				case len(upgradePath.Steps) == 0 && upgradePath.AllComponentsReady:
					// upgrade fully completed
					upgradeStatus.TypedSpec().Value.Phase = specs.KubernetesUpgradeStatusSpec_Done
					upgradeStatus.TypedSpec().Value.Step = ""
					upgradeStatus.TypedSpec().Value.Status = ""
					upgradeStatus.TypedSpec().Value.Error = ""
					upgradeStatus.TypedSpec().Value.LastUpgradeVersion = cluster.TypedSpec().Value.KubernetesVersion
					upgradeStatus.TypedSpec().Value.CurrentUpgradeVersion = ""
				case len(upgradePath.Steps) > 0 && !versionMismatch && upgradeStatus.TypedSpec().Value.Phase != specs.KubernetesUpgradeStatusSpec_Done:
					// reverting an upgrade (?)
					var applied bool

					applied, err = applyUpgradePatches(ctx, r, cluster, upgradePath.Steps...)
					if err != nil {
						return err
					}

					if applied {
						upgradeStatus.TypedSpec().Value.Phase = specs.KubernetesUpgradeStatusSpec_Reverting
						upgradeStatus.TypedSpec().Value.Step = "reverting the change"
						upgradeStatus.TypedSpec().Value.Status = ""
						upgradeStatus.TypedSpec().Value.Error = ""
					}
				case len(upgradePath.Steps) > 0 && upgradePath.AllComponentsReady:
					prePullStatus, prePullDone, prePullErr := updateImagePullRequest(ctx, r, upgradePath)
					if prePullErr != nil {
						return prePullErr
					}

					if !prePullDone {
						if prePullStatus == nil {
							return nil // there is no ImagePullStatus for the current version of the ImagePullRequest yet
						}

						processedError := prePullStatus.TypedSpec().Value.GetLastProcessedError()
						if processedError != "" {
							upgradeStatus.TypedSpec().Value.Step = fmt.Sprintf("failed to pull %q", prePullStatus.TypedSpec().Value.GetLastProcessedImage())
						} else {
							upgradeStatus.TypedSpec().Value.Step = fmt.Sprintf("pulled %q", prePullStatus.TypedSpec().Value.GetLastProcessedImage())
						}

						upgradeStatus.TypedSpec().Value.Phase = specs.KubernetesUpgradeStatusSpec_Upgrading
						upgradeStatus.TypedSpec().Value.Status = fmt.Sprintf("pre-pulling images (%d/%d)",
							prePullStatus.TypedSpec().Value.GetProcessedCount(),
							prePullStatus.TypedSpec().Value.GetTotalCount())
						upgradeStatus.TypedSpec().Value.Error = processedError

						return nil
					}

					// ready to apply the next patch
					patch := upgradePath.Steps[0]

					var skip bool

					skip, err = skipLocked(ctx, r, patch, upgradeStatus)
					if err != nil {
						return err
					}

					if skip {
						return nil
					}

					upgradeStatus.TypedSpec().Value.Phase = specs.KubernetesUpgradeStatusSpec_Upgrading
					upgradeStatus.TypedSpec().Value.Step = patch.Description
					upgradeStatus.TypedSpec().Value.Status = "waiting for a restart"
					upgradeStatus.TypedSpec().Value.Error = ""

					if _, err = applyUpgradePatches(ctx, r, cluster, patch); err != nil {
						return err
					}
				case !upgradePath.AllComponentsReady:
					upgradeStatus.TypedSpec().Value.Status = upgradePath.NotReadyStatus
					upgradeStatus.TypedSpec().Value.Error = ""
				default:
					upgradeStatus.TypedSpec().Value.Phase = specs.KubernetesUpgradeStatusSpec_Unknown
				}

				if upgradeStatus.TypedSpec().Value.Phase == specs.KubernetesUpgradeStatusSpec_Done {
					// upgrade is done, so calculate the upgrade versions
					upgradeStatus.TypedSpec().Value.UpgradeVersions, err = kubernetes.CalculateUpgradeVersions(
						ctx, r, upgradeStatus.TypedSpec().Value.LastUpgradeVersion, cluster.TypedSpec().Value.TalosVersion)
					if err != nil {
						return err
					}
				} else {
					// upgrade is going, or failed, so clear the upgrade versions
					upgradeStatus.TypedSpec().Value.UpgradeVersions = nil
				}

				return nil
			},
			FinalizerRemovalExtraOutputFunc: func(ctx context.Context, r controller.ReaderWriter, _ *zap.Logger, cluster *omni.Cluster) error {
				// drop any config patches created by this controller
				configPatches, err := safe.ReaderListAll[*omni.ConfigPatch](ctx, r, state.WithLabelQuery(
					resource.LabelEqual(omni.LabelCluster, cluster.Metadata().ID()),
				))
				if err != nil {
					return err
				}

				for cp := range configPatches.All() {
					if cp.Metadata().Owner() == KubernetesUpgradeStatusControllerName {
						if err = r.Destroy(ctx, cp.Metadata()); err != nil {
							return err
						}
					}
				}

				// drop any ImagePullRequests created by this controller
				imagePullRequests, err := safe.ReaderListAll[*omni.ImagePullRequest](ctx, r, state.WithLabelQuery(
					resource.LabelEqual(omni.LabelCluster, cluster.Metadata().ID()),
				))
				if err != nil {
					return err
				}

				for ipr := range imagePullRequests.All() {
					if ipr.Metadata().Owner() == KubernetesUpgradeStatusControllerName {
						if err = r.Destroy(ctx, ipr.Metadata()); err != nil {
							return err
						}
					}
				}

				return nil
			},
		},
		qtransform.WithExtraMappedInput(
			qtransform.MapperSameID[*omni.ClusterStatus, *omni.Cluster](),
		),
		qtransform.WithExtraMappedInput(
			qtransform.MapperSameID[*omni.KubernetesStatus, *omni.Cluster](),
		),
		qtransform.WithExtraMappedInput(
			qtransform.MapperSameID[*omni.ImagePullStatus, *omni.Cluster](),
		),
		qtransform.WithExtraMappedInput(
			func(ctx context.Context, _ *zap.Logger, r controller.QRuntime, _ *omni.KubernetesVersion) ([]resource.Pointer, error) {
				// reconcile all cluster on KubernetesVersion changes
				clusters, err := safe.ReaderListAll[*omni.Cluster](ctx, r)
				if err != nil {
					return nil, err
				}

				return safe.Map(clusters, func(cluster *omni.Cluster) (resource.Pointer, error) {
					return cluster.Metadata(), nil
				})
			},
		),
		qtransform.WithExtraMappedInput(
			mappers.MapByClusterLabel[*omni.ClusterMachineIdentity, *omni.Cluster](),
		),
		qtransform.WithExtraMappedInput(
			mappers.MapByClusterLabel[*omni.MachineSetNode, *omni.Cluster](),
		),
		qtransform.WithExtraOutputs(
			controller.Output{
				Type: omni.ConfigPatchType,
				Kind: controller.OutputShared,
			},
			controller.Output{
				Type: omni.ImagePullRequestType,
				Kind: controller.OutputShared,
			},
		),
		qtransform.WithConcurrency(2),
	)
}

func updateImagePullRequest(ctx context.Context, r controller.ReaderWriter, upgradePath *kubernetes.UpgradePath) (sts *omni.ImagePullStatus, done bool, err error) {
	request, err := safe.WriterModifyWithResult[*omni.ImagePullRequest](ctx, r, omni.NewImagePullRequest(resources.DefaultNamespace, upgradePath.ClusterID), func(r *omni.ImagePullRequest) error {
		var nodeImageList []*specs.ImagePullRequestSpec_NodeImageList

		for _, node := range upgradePath.AllNodes {
			nodeImageList = append(nodeImageList,
				&specs.ImagePullRequestSpec_NodeImageList{
					Node:   node,
					Images: upgradePath.AllNodesToRequiredImages[node],
				},
			)
		}

		r.Metadata().Labels().Set(omni.LabelCluster, upgradePath.ClusterID)

		r.TypedSpec().Value.NodeImageList = nodeImageList

		return nil
	})
	if err != nil {
		return nil, false, fmt.Errorf("failed to update ImagePullRequest: %w", err)
	}

	sts, err = safe.ReaderGet[*omni.ImagePullStatus](ctx, r, omni.NewImagePullStatus(resources.DefaultNamespace, upgradePath.ClusterID).Metadata())
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, false, nil
		}

		return nil, false, fmt.Errorf("failed to get ImagePullStatus: %w", err)
	}

	versionMatch := request.Metadata().Version().String() == sts.TypedSpec().Value.GetRequestVersion()
	if !versionMatch {
		return nil, false, nil
	}

	done = sts.TypedSpec().Value.GetProcessedCount() == sts.TypedSpec().Value.GetTotalCount()

	return sts, done, nil
}

func applyUpgradePatches(ctx context.Context, r controller.Writer, cluster *omni.Cluster, patches ...kubernetes.UpgradeStep) (bool, error) {
	anyPatchApplied := false

	for _, patch := range patches {
		if err := safe.WriterModify(ctx, r,
			omni.NewConfigPatch(resources.DefaultNamespace, fmt.Sprintf("900-cm-%s-kubernetes-upgrade", patch.MachineID)),
			func(configPatch *omni.ConfigPatch) error {
				var cfg v1alpha1.Config

				buffer, err := configPatch.TypedSpec().Value.GetUncompressedData()
				if err != nil {
					return err
				}

				defer buffer.Free()

				patchData := buffer.Data()

				if len(patchData) > 0 {
					if err = yaml.Unmarshal(patchData, &cfg); err != nil {
						return err
					}
				}

				oldCfg := cfg.DeepCopy()

				patch.Patch.Apply(&cfg)

				// patch is already applied, skip it
				if reflect.DeepEqual(oldCfg, cfg) {
					return nil
				}

				configPatch.Metadata().Labels().Set(omni.LabelCluster, cluster.Metadata().ID())
				configPatch.Metadata().Labels().Set(omni.LabelClusterMachine, patch.MachineID)
				configPatch.Metadata().Labels().Set(omni.LabelSystemPatch, "")

				data, err := yaml.Marshal(cfg)
				if err != nil {
					return err
				}

				if err = configPatch.TypedSpec().Value.SetUncompressedData(data); err != nil {
					return err
				}

				anyPatchApplied = true

				return nil
			}); err != nil {
			return false, err
		}
	}

	return anyPatchApplied, nil
}

func skipLocked(ctx context.Context, r controller.Reader, patch kubernetes.UpgradeStep, upgradeStatus *omni.KubernetesUpgradeStatus) (bool, error) {
	machineSetNode, err := r.Get(ctx, resource.NewMetadata(resources.DefaultNamespace, omni.MachineSetNodeType, patch.MachineID, resource.VersionUndefined))
	if err != nil && !state.IsNotFoundError(err) {
		return false, err
	}

	if machineSetNode != nil {
		if _, locked := machineSetNode.Metadata().Annotations().Get(omni.MachineLocked); locked {
			upgradeStatus.TypedSpec().Value.Phase = specs.KubernetesUpgradeStatusSpec_Upgrading
			upgradeStatus.TypedSpec().Value.Step = patch.Description
			upgradeStatus.TypedSpec().Value.Status = "waiting for machine to be unlocked"
			upgradeStatus.TypedSpec().Value.Error = ""

			return true, nil
		}
	}

	return false, nil
}
