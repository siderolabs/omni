// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"text/template"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/hashicorp/go-multierror"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

//go:embed data/kube-service-exposer-config-patch.tmpl.yaml
var kubeServiceExposerConfigPatchTemplate string

// ClusterWorkloadProxyController is a controller that manages cluster workload proxy setting.
type ClusterWorkloadProxyController struct {
	configPatchData []byte
}

// Name returns the name of the controller.
func (ctrl *ClusterWorkloadProxyController) Name() string {
	return "ClusterWorkloadProxyController"
}

// Inputs implements controller.Controller interface.
func (ctrl *ClusterWorkloadProxyController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: resources.DefaultNamespace,
			Type:      omni.ClusterType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *ClusterWorkloadProxyController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: omni.ConfigPatchType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *ClusterWorkloadProxyController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		tracker := trackResource(r, resources.DefaultNamespace, omni.ConfigPatchType)

		tracker.owner = ctrl.Name()

		clusterList, err := safe.ReaderListAll[*omni.Cluster](ctx, r)
		if err != nil {
			return fmt.Errorf("failed to list clusters: %w", err)
		}

		configPatchData, err := ctrl.getConfigPatchData()
		if err != nil {
			return fmt.Errorf("failed to get config patch: %w", err)
		}

		var errs error

		for cluster := range clusterList.All() {
			id := fmt.Sprintf("950-cluster-%s-workload-proxying", cluster.Metadata().ID())

			configPatch := omni.NewConfigPatch(id)

			if !cluster.TypedSpec().Value.GetFeatures().GetEnableWorkloadProxy() {
				if destroyErr := r.Destroy(ctx, configPatch.Metadata()); destroyErr != nil && !state.IsNotFoundError(destroyErr) {
					errs = multierror.Append(errs, fmt.Errorf("failed to destroy config patch: %w", destroyErr))
				}

				continue
			}

			if modifyErr := safe.WriterModify(ctx, r, configPatch, func(patch *omni.ConfigPatch) error {
				patch.Metadata().Labels().Set(omni.LabelCluster, cluster.Metadata().ID())
				patch.Metadata().Labels().Set(omni.LabelSystemPatch, "")

				return patch.TypedSpec().Value.SetUncompressedData(configPatchData)
			}); modifyErr != nil {
				errs = multierror.Append(errs, fmt.Errorf("failed to modify config patch: %w", modifyErr))
			}

			tracker.keep(configPatch)
		}

		if errs != nil {
			return errs
		}

		if err = tracker.cleanup(ctx); err != nil {
			return err
		}

		r.ResetRestartBackoff()
	}
}

func (ctrl *ClusterWorkloadProxyController) getConfigPatchData() ([]byte, error) {
	if len(ctrl.configPatchData) > 0 {
		return ctrl.configPatchData, nil
	}

	tmpl, err := template.New("kube-service-exposer-config-patch").Parse(kubeServiceExposerConfigPatchTemplate)
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

	ctrl.configPatchData = buf.Bytes()

	return ctrl.configPatchData, nil
}
