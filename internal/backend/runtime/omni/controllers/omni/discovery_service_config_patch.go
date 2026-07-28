// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/siderolabs/gen/xerrors"
	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	configcluster "github.com/siderolabs/talos/pkg/machinery/config/types/cluster"
	"go.uber.org/zap"
	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/pkg/siderolink"
)

// DiscoveryServiceConfigPatchPrefix is the prefix for the system config patch that contains the discovery service endpoint config.
const DiscoveryServiceConfigPatchPrefix = "950-discovery-service-"

// Names of the DiscoveryServiceConfig document (Talos 1.14+) that points at the embedded discovery service.
// For embedded-only it reuses "default" to override the base public document. When public discovery is kept
// as well it is added under a distinct name alongside the untouched base "default" document.
const (
	embeddedOnlyDiscoveryServiceConfigName      = "default"
	embeddedAndPublicDiscoveryServiceConfigName = "omni-embedded"
)

// DiscoveryServiceConfigPatchController creates the system config patch that contains the discovery service endpoint config.
//
// It points the cluster at the embedded discovery service when the feature is available for the Omni instance and enabled
// for the cluster, optionally keeping the public discovery service alongside it on Talos 1.14+.
type DiscoveryServiceConfigPatchController struct {
	*qtransform.QController[*omni.ClusterStatus, *omni.ConfigPatch]

	embeddedEndpoint string
}

// NewDiscoveryServiceConfigPatchController initializes DiscoveryServiceConfigPatchController.
func NewDiscoveryServiceConfigPatchController(embeddedDiscoveryServicePort int) *DiscoveryServiceConfigPatchController {
	ctrl := &DiscoveryServiceConfigPatchController{
		embeddedEndpoint: "http://" + net.JoinHostPort(siderolink.ListenHost, strconv.Itoa(embeddedDiscoveryServicePort)),
	}

	ctrl.QController = qtransform.NewQController(
		qtransform.Settings[*omni.ClusterStatus, *omni.ConfigPatch]{
			Name: "DiscoveryServiceConfigPatchController",
			MapMetadataFunc: func(cluster *omni.ClusterStatus) *omni.ConfigPatch {
				return omni.NewConfigPatch(DiscoveryServiceConfigPatchPrefix + cluster.Metadata().ID())
			},
			UnmapMetadataFunc: func(configPatch *omni.ConfigPatch) *omni.ClusterStatus {
				id := strings.TrimPrefix(configPatch.Metadata().ID(), DiscoveryServiceConfigPatchPrefix)

				return omni.NewClusterStatus(id)
			},
			TransformExtraOutputFunc: ctrl.reconcile,
		},
		qtransform.WithConcurrency(2),
		qtransform.WithOutputKind(controller.OutputShared),
	)

	return ctrl
}

func (ctrl *DiscoveryServiceConfigPatchController) reconcile(_ context.Context, _ controller.ReaderWriter, _ *zap.Logger, cluster *omni.ClusterStatus, configPatch *omni.ConfigPatch) error {
	configPatch.Metadata().Labels().Set(omni.LabelSystemPatch, "")
	configPatch.Metadata().Labels().Set(omni.LabelCluster, cluster.Metadata().ID())

	useEmbedded := cluster.TypedSpec().Value.UseEmbeddedDiscoveryService
	disablePublic := cluster.TypedSpec().Value.DisablePublicDiscoveryService

	if !useEmbedded && !disablePublic {
		// public only is the base config default on both the legacy and the multi-doc format, no patch needed
		return xerrors.NewTaggedf[qtransform.DestroyOutputTag]("cluster uses the default public discovery service")
	}

	initialTalosVersion := cluster.TypedSpec().Value.InitialTalosVersion
	if initialTalosVersion == "" {
		// not resolved yet, the ClusterStatus update re-triggers reconcile once it is set
		return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("cluster initial Talos version is not set yet")
	}

	vc, err := config.ParseContractFromVersion(initialTalosVersion)
	if err != nil {
		return fmt.Errorf("failed to parse Talos version contract: %w", err)
	}

	multidoc := vc.DiscoveryServiceMultidocConfig()

	var patchYAML []byte

	switch {
	case !multidoc:
		// legacy base config: the only non-default case here is embedded-only (public can't be turned off on < 1.14)
		patchYAML, err = ctrl.buildLegacyDiscoveryPatch()
	case !useEmbedded && disablePublic:
		// multi-doc, everything off: remove the base public "default" document so no discovery service remains
		patchYAML, err = ctrl.buildDisablePublicDiscoveryPatch()
	case useEmbedded && !disablePublic:
		// multi-doc, both: add an embedded endpoint alongside the base public "default" document
		patchYAML, err = ctrl.buildDiscoveryServiceDocumentPatch(embeddedAndPublicDiscoveryServiceConfigName)
	default:
		// multi-doc, embedded only: override the base public "default" document to point only at the embedded service
		patchYAML, err = ctrl.buildDiscoveryServiceDocumentPatch(embeddedOnlyDiscoveryServiceConfigName)
	}

	if err != nil {
		return err
	}

	return configPatch.TypedSpec().Value.SetUncompressedData(patchYAML)
}

// buildLegacyDiscoveryPatch builds the deprecated single-endpoint discovery patch used on Talos < 1.14.
func (ctrl *DiscoveryServiceConfigPatchController) buildLegacyDiscoveryPatch() ([]byte, error) {
	var buf bytes.Buffer

	enc := yaml.NewEncoder(&buf)

	enc.SetIndent(2)

	if err := enc.Encode(map[string]any{
		"cluster": map[string]any{
			"discovery": map[string]any{
				"registries": map[string]any{
					"service": map[string]any{
						"endpoint": ctrl.embeddedEndpoint,
					},
				},
			},
		},
	}); err != nil {
		return nil, fmt.Errorf("failed to encode patch: %w", err)
	}

	return buf.Bytes(), nil
}

// buildDiscoveryServiceDocumentPatch builds a DiscoveryServiceConfig document patch (Talos 1.14+) with the given name.
func (ctrl *DiscoveryServiceConfigPatchController) buildDiscoveryServiceDocumentPatch(name string) ([]byte, error) {
	endpointURL, err := url.Parse(ctrl.embeddedEndpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to parse embedded discovery service endpoint %q: %w", ctrl.embeddedEndpoint, err)
	}

	document := configcluster.NewDiscoveryServiceConfigV1Alpha1(name, endpointURL)

	ctr, err := container.New(document)
	if err != nil {
		return nil, fmt.Errorf("failed to build discovery service config document: %w", err)
	}

	return ctr.EncodeBytes(encoder.WithComments(encoder.CommentsDisabled))
}

// buildDisablePublicDiscoveryPatch builds the patch that turns the public discovery service off on the Talos 1.14+
// multi-doc format, where the base config carries a DiscoveryServiceConfig "default" document.
func (ctrl *DiscoveryServiceConfigPatchController) buildDisablePublicDiscoveryPatch() ([]byte, error) {
	var buf bytes.Buffer

	enc := yaml.NewEncoder(&buf)

	enc.SetIndent(2)

	if err := enc.Encode(map[string]any{
		"apiVersion": "v1alpha1",
		"kind":       configcluster.DiscoveryServiceKind,
		"name":       embeddedOnlyDiscoveryServiceConfigName,
		"$patch":     "delete",
	}); err != nil {
		return nil, fmt.Errorf("failed to encode patch: %w", err)
	}

	return buf.Bytes(), nil
}
