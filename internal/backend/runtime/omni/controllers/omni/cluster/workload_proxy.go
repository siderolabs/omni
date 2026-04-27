// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package cluster

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"text/template"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/siderolabs/gen/xerrors"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

//go:embed data/kube-service-exposer-manifest.tmpl.yaml
var kubeServiceExposerManifestTemplate string

// ClusterWorkloadProxyController is a controller that manages cluster workload proxy setting.
type ClusterWorkloadProxyController struct {
	*qtransform.QController[*omni.Cluster, *omni.KubernetesManifestGroup]
	manifestData []byte
}

// NewClusterWorkloadProxyController creates a new cluster workload proxy controller.
func NewClusterWorkloadProxyController(workloadProxyEnabled bool) (*ClusterWorkloadProxyController, error) {
	ctrl := &ClusterWorkloadProxyController{}

	if workloadProxyEnabled {
		manifestData, err := getManifestData()
		if err != nil {
			return nil, fmt.Errorf("failed to get manifest data: %w", err)
		}

		ctrl.manifestData = manifestData
	}

	ctrl.QController = qtransform.NewQController(
		qtransform.Settings[*omni.Cluster, *omni.KubernetesManifestGroup]{
			Name: "ClusterWorkloadProxyController",
			MapMetadataFunc: func(cluster *omni.Cluster) *omni.KubernetesManifestGroup {
				id := fmt.Sprintf("cluster-%s-workload-proxy", cluster.Metadata().ID())

				return omni.NewKubernetesManifestGroup(id)
			},
			UnmapMetadataFunc: func(km *omni.KubernetesManifestGroup) *omni.Cluster {
				cluster, _ := km.Metadata().Labels().Get(omni.LabelCluster)

				return omni.NewCluster(cluster)
			},
			TransformFunc: ctrl.transform,
		},
		qtransform.WithOutputKind(controller.OutputShared),
	)

	return ctrl, nil
}

func (ctrl *ClusterWorkloadProxyController) transform(_ context.Context, _ controller.Reader, logger *zap.Logger,
	cluster *omni.Cluster, kubernetesManifestGroup *omni.KubernetesManifestGroup,
) error {
	if ctrl.manifestData == nil || !cluster.TypedSpec().Value.GetFeatures().GetEnableWorkloadProxy() {
		return xerrors.NewTaggedf[qtransform.DestroyOutputTag]("workload proxy is disabled for the cluster %s", cluster.Metadata().ID())
	}

	kubernetesManifestGroup.Metadata().Labels().Set(omni.LabelCluster, cluster.Metadata().ID())
	kubernetesManifestGroup.Metadata().Labels().Set(omni.LabelSystemManifest, "")
	kubernetesManifestGroup.TypedSpec().Value.Mode = specs.KubernetesManifestGroupSpec_FULL

	return kubernetesManifestGroup.TypedSpec().Value.SetUncompressedData(ctrl.manifestData)
}

func getManifestData() ([]byte, error) {
	tmpl, err := template.New("kube-service-exposer-manifests").Parse(kubeServiceExposerManifestTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}

	type tmplOptions struct {
		AnnotationKey string
	}

	opts := tmplOptions{
		AnnotationKey: constants.ExposedServicePortAnnotationKey,
	}

	var buf bytes.Buffer

	if err = tmpl.Execute(&buf, opts); err != nil {
		return nil, fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.Bytes(), nil
}
