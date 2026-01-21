// Copyright (c) 2026 Sidero Labs, Inc.
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
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
)

//go:embed data/kms-encryption-config-patch.yaml
var encryptionConfigPatchData string

// ClusterMachineEncryptionController manages disk encryption configuration for the cluster.
type ClusterMachineEncryptionController struct{}

// Name implements controller.Controller interface.
func (*ClusterMachineEncryptionController) Name() string {
	return "ClusterMachineEncryptionController"
}

// Inputs implements controller.Controller interface.
func (*ClusterMachineEncryptionController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: resources.DefaultNamespace,
			Type:      omni.ClusterType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: resources.DefaultNamespace,
			Type:      siderolink.APIConfigType,
			ID:        optional.Some(siderolink.ConfigID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (*ClusterMachineEncryptionController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: omni.ConfigPatchType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *ClusterMachineEncryptionController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		tracker := trackResource(r, resources.DefaultNamespace, omni.ConfigPatchType)
		tracker.owner = ctrl.Name()

		clusters, err := safe.ReaderListAll[*omni.Cluster](ctx, r)
		if err != nil {
			return err
		}

		params, err := safe.ReaderGetByID[*siderolink.APIConfig](ctx, r, siderolink.ConfigID)
		if err != nil {
			return err
		}

		kmsEndpoint := params.TypedSpec().Value.MachineApiAdvertisedUrl

		template, err := template.New("patch").Parse(encryptionConfigPatchData)
		if err != nil {
			return err
		}

		var buffer bytes.Buffer

		if err = template.Execute(&buffer, struct {
			KmsEndpoint string
		}{
			KmsEndpoint: kmsEndpoint,
		}); err != nil {
			return err
		}

		data := buffer.Bytes()

		err = clusters.ForEachErr(func(cluster *omni.Cluster) error {
			if !omni.GetEncryptionEnabled(cluster) {
				return nil
			}

			clusterName := cluster.Metadata().ID()

			patch := omni.NewConfigPatch(
				fmt.Sprintf("%s-%s-encryption", constants.EncryptionPatchPrefix, clusterName),
			)

			return safe.WriterModify[*omni.ConfigPatch](ctx, r, patch, func(r *omni.ConfigPatch) error {
				if err = r.TypedSpec().Value.SetUncompressedData(data); err != nil {
					return err
				}

				r.Metadata().Annotations().Set(omni.ConfigPatchName, constants.EncryptionConfigName)
				r.Metadata().Annotations().Set(omni.ConfigPatchDescription, constants.EncryptionConfigDescription)
				r.Metadata().Labels().Set(omni.LabelCluster, clusterName)
				r.Metadata().Labels().Set(omni.LabelSystemPatch, "")

				tracker.keep(r)

				return nil
			})
		})
		if err != nil {
			return err
		}

		if err = tracker.cleanup(ctx); err != nil {
			return err
		}
	}
}
