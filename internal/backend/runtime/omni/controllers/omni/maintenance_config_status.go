// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xerrors"
	machine "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/config"
	talosconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/configpatcher"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
	configres "github.com/siderolabs/talos/pkg/machinery/resources/config"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	siderolinkres "github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/client/pkg/siderolink"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/imagefactoryauth"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/uncached"
	"github.com/siderolabs/omni/internal/backend/runtime/talos"
	omnicfg "github.com/siderolabs/omni/internal/pkg/config"
)

// MaintenanceClientFactory creates a MaintenanceClient for the given machine.
type MaintenanceClientFactory = func(ctx context.Context, machineID string) (MaintenanceClient, error)

// MaintenanceClient is a client for interacting with Talos running in maintenance mode.
type MaintenanceClient interface {
	GetMachineConfig(ctx context.Context) (*configres.MachineConfig, error)
	ApplyConfiguration(ctx context.Context, req *machine.ApplyConfigurationRequest) (*machine.ApplyConfigurationResponse, error)
}

// NewMaintenanceClientFactory returns a MaintenanceClientFactory backed by a Talos client factory.
//
// It uses exclusively maintenance mode Talos clients.
func NewMaintenanceClientFactory(clientFactory *talos.ClientFactory) MaintenanceClientFactory {
	return func(ctx context.Context, machineID string) (MaintenanceClient, error) {
		talosClient, err := clientFactory.GetMaintenance(ctx, machineID)
		if err != nil {
			return nil, err
		}

		return maintenanceClient{client: talosClient}, nil
	}
}

type maintenanceClient struct {
	client *talos.Client
}

func (c maintenanceClient) GetMachineConfig(ctx context.Context) (*configres.MachineConfig, error) {
	machineConfig, err := safe.ReaderGetByID[*configres.MachineConfig](ctx, c.client.COSI, configres.ActiveID)
	if err != nil && !state.IsNotFoundError(err) {
		return nil, fmt.Errorf("error getting machine config: %w", err)
	}

	return machineConfig, nil
}

func (c maintenanceClient) ApplyConfiguration(ctx context.Context, req *machine.ApplyConfigurationRequest) (*machine.ApplyConfigurationResponse, error) {
	return c.client.ApplyConfiguration(ctx, req)
}

// MaintenanceConfigStatusController manages MaintenanceConfigStatus resource lifecycle.
//
// MaintenanceConfigStatusController generates cluster UUID for every cluster.
type MaintenanceConfigStatusController = qtransform.QController[*siderolinkres.Link, *omni.MaintenanceConfigStatus]

// NewMaintenanceConfigStatusController initializes MaintenanceConfigStatusController.
func NewMaintenanceConfigStatusController(maintenanceClientFactory MaintenanceClientFactory, eventSinkPort, logServerPort int, registries omnicfg.Registries) *MaintenanceConfigStatusController {
	helper := newMaintenanceConfigStatusControllerHelper(maintenanceClientFactory, eventSinkPort, logServerPort, registries)

	return qtransform.NewQController(
		qtransform.Settings[*siderolinkres.Link, *omni.MaintenanceConfigStatus]{
			Name: "MaintenanceConfigStatusController",
			MapMetadataFunc: func(link *siderolinkres.Link) *omni.MaintenanceConfigStatus {
				return omni.NewMaintenanceConfigStatus(link.Metadata().ID())
			},
			UnmapMetadataFunc: func(status *omni.MaintenanceConfigStatus) *siderolinkres.Link {
				return siderolinkres.NewLink(status.Metadata().ID(), nil)
			},
			TransformFunc: helper.transform,
		},
		qtransform.WithExtraMappedInput[*omni.MachineStatus](
			qtransform.MapperSameID[*siderolinkres.Link](),
		),
		qtransform.WithExtraMappedInput[*omni.MachineConfigExtractionStatus](
			qtransform.MapperSameID[*siderolinkres.Link](),
		),
		qtransform.WithExtraMappedInput[*omni.ConfigPatch](
			func(_ context.Context, _ *zap.Logger, _ controller.QRuntime, patch controller.ReducedResourceMetadata) ([]resource.Pointer, error) {
				// only machine-level config patches are relevant in maintenance mode
				machineID, ok := patch.Labels().Get(omni.LabelMachine)
				if !ok {
					return nil, nil
				}

				return []resource.Pointer{siderolinkres.NewLink(machineID, nil).Metadata()}, nil
			},
		),
		qtransform.WithConcurrency(32),
	)
}

// maintenanceBasePatch is the Omni-managed base config applied in maintenance mode, together with its rendered bytes.
type maintenanceBasePatch struct {
	patch configpatcher.Patch
	data  []byte
}

type maintenanceConfigStatusControllerHelper struct {
	getMaintenanceConfigPatch func(talosVersion string) (maintenanceBasePatch, error)
	maintenanceClientFactory  MaintenanceClientFactory
}

func newMaintenanceConfigStatusControllerHelper(maintenanceClientFactory MaintenanceClientFactory, eventSinkPort, logServerPort int, registries omnicfg.Registries,
) *maintenanceConfigStatusControllerHelper {
	buildPatch := func(extraDocs ...talosconfig.Document) (maintenanceBasePatch, error) {
		cfg, err := siderolink.NewJoinOptions(
			siderolink.WithoutMachineAPIURL(),
			siderolink.WithEventSinkPort(eventSinkPort),
			siderolink.WithLogServerPort(logServerPort),
		)
		if err != nil {
			return maintenanceBasePatch{}, err
		}

		configBytes, err := cfg.RenderJoinConfig(extraDocs...)
		if err != nil {
			return maintenanceBasePatch{}, err
		}

		patch, err := configpatcher.LoadPatch(configBytes)
		if err != nil {
			return maintenanceBasePatch{}, fmt.Errorf("failed to load patch: %w", err)
		}

		return maintenanceBasePatch{patch: patch, data: configBytes}, nil
	}

	basePatch := sync.OnceValues(func() (maintenanceBasePatch, error) {
		return buildPatch()
	})

	patchWithRegistryAuth := sync.OnceValues(func() (maintenanceBasePatch, error) {
		authDoc, err := imagefactoryauth.BuildDoc(registries)
		if err != nil {
			return maintenanceBasePatch{}, fmt.Errorf("error building registry auth doc: %w", err)
		}

		if authDoc == nil {
			return basePatch()
		}

		return buildPatch(authDoc)
	})

	return &maintenanceConfigStatusControllerHelper{
		maintenanceClientFactory: maintenanceClientFactory,
		getMaintenanceConfigPatch: func(talosVersion string) (maintenanceBasePatch, error) {
			vc, err := config.ParseContractFromVersion(talosVersion)
			if err != nil {
				return maintenanceBasePatch{}, fmt.Errorf("failed to parse contract from version: %w", err)
			}

			if vc.MultidocNetworkConfigSupported() {
				return patchWithRegistryAuth()
			}

			return basePatch()
		},
	}
}

//nolint:gocyclo,cyclop
func (helper *maintenanceConfigStatusControllerHelper) transform(ctx context.Context, r controller.Reader, logger *zap.Logger, link *siderolinkres.Link, status *omni.MaintenanceConfigStatus) error {
	if !link.TypedSpec().Value.Connected {
		return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("machine is not connected")
	}

	machineStatus, err := safe.ReaderGetByID[*omni.MachineStatus](ctx, r, link.Metadata().ID())
	if err != nil {
		if state.IsNotFoundError(err) {
			return xerrors.NewTagged[qtransform.SkipReconcileTag](err)
		}

		return err
	}

	if !machineStatus.TypedSpec().Value.Maintenance {
		return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("machine is not in maintenance mode")
	}

	if machineStatus.TypedSpec().Value.PowerState == specs.MachineStatusSpec_POWER_STATE_OFF {
		return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("machine is powered off, skip maintenance config update")
	}

	if machineStatus.TypedSpec().Value.Schematic.GetInAgentMode() {
		return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("machine is in agent mode, cannot apply config, skip")
	}

	talosVersion := machineStatus.TypedSpec().Value.TalosVersion
	if talosVersion == "" {
		return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("machine has no talos version yet")
	}

	if !quirks.New(talosVersion).SupportsMultidoc() {
		return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("talos version does not support multidoc, nothing to do")
	}

	// wait for the machine's incoming config to be extracted before applying anything, otherwise we could overwrite (and lose) the config the machine arrived with before it is preserved.
	// read uncached, to avoid acting on a stale (or not yet visible) extraction status.
	extractionStatus, err := safe.ReaderGetByID[*omni.MachineConfigExtractionStatus](ctx, uncached.Reader(r), link.Metadata().ID())
	if err != nil && !state.IsNotFoundError(err) {
		return err
	}

	if extractionStatus == nil || !extractionStatus.TypedSpec().Value.Initialized {
		return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("machine config has not been extracted yet")
	}

	// read machine config patches uncached, to avoid acting on a stale (or not yet visible) just-extracted preserved config patch
	machinePatches, err := getMachinePatches(ctx, uncached.Reader(r), link.Metadata().ID())
	if err != nil {
		return fmt.Errorf("error collecting machine config patches: %w", err)
	}

	baseConfig, err := helper.getMaintenanceConfigPatch(talosVersion)
	if err != nil {
		return fmt.Errorf("error building machine config: %w", err)
	}

	// re-apply when the machine rebooted/reconnected (the public key changes, and maintenance config is not persisted across reboots)
	// or when the desired config (the machine config patches or the Omni-managed base config) changed
	desiredHash := desiredConfigHash(machinePatches, baseConfig.data)
	if link.TypedSpec().Value.NodePublicKey == status.TypedSpec().Value.PublicKeyAtLastApply &&
		desiredHash == status.TypedSpec().Value.LastAppliedConfigHash {
		return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("public key and desired maintenance config unchanged, skip")
	}

	maintenanceTalosClient, err := helper.maintenanceClientFactory(ctx, machineStatus.Metadata().ID())
	if err != nil {
		return fmt.Errorf("error creating maintenance client: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	maintenanceConfig, err := maintenanceTalosClient.GetMachineConfig(ctx)
	if err != nil {
		return fmt.Errorf("error getting maintenance config: %w", err)
	}

	var machineConfig config.Provider

	if maintenanceConfig != nil {
		machineConfig = maintenanceConfig.Provider()
	} else if machineConfig, err = container.New(); err != nil {
		return fmt.Errorf("error creating new config container: %w", err)
	}

	patches, err := configpatcher.LoadPatches(machinePatches)
	if err != nil {
		return fmt.Errorf("error loading machine config patches: %w", err)
	}

	// the Omni-managed connection documents are applied last, so they always win over anything in the user patches.
	// the machine's own SideroLink document (if any) is preserved as-is by the merge, so its connection is never overwritten or broken.
	patches = append(patches, baseConfig.patch)

	patched, err := configpatcher.Apply(configpatcher.WithConfig(machineConfig), patches)
	if err != nil {
		return fmt.Errorf("error applying patches: %w", err)
	}

	patchedConfig, err := patched.Config()
	if err != nil {
		return fmt.Errorf("error reading patched config: %w", err)
	}

	// never apply a v1alpha1 document in maintenance mode
	strippedConfig, err := stripV1Alpha1(patchedConfig)
	if err != nil {
		return fmt.Errorf("error stripping v1alpha1 document: %w", err)
	}

	patchedBytes, err := strippedConfig.EncodeBytes(encoder.WithComments(encoder.CommentsDisabled))
	if err != nil {
		return fmt.Errorf("error encoding patched config: %w", err)
	}

	if _, err = maintenanceTalosClient.ApplyConfiguration(ctx, &machine.ApplyConfigurationRequest{
		Data: patchedBytes,
		Mode: machine.ApplyConfigurationRequest_AUTO,
	}); err != nil {
		if grpcstatus.Code(err) == codes.Unimplemented {
			return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("machine does not support applying configuration: %w", err)
		}

		return fmt.Errorf("error applying maintenance config: %w", err)
	}

	logger.Info("applied maintenance config")

	status.TypedSpec().Value.PublicKeyAtLastApply = link.TypedSpec().Value.NodePublicKey
	status.TypedSpec().Value.LastAppliedConfigHash = desiredHash

	return nil
}

// getMachinePatches collects the running machine-level config patches for the machine, ordered by ID (lower ID = lower priority).
func getMachinePatches(ctx context.Context, r controller.Reader, machineID resource.ID) ([]string, error) {
	list, err := safe.ReaderListAll[*omni.ConfigPatch](ctx, r, state.WithLabelQuery(
		resource.LabelEqual(omni.LabelMachine, machineID),
		resource.LabelExists(omni.LabelDisabled, resource.NotMatches),
	))
	if err != nil {
		return nil, err
	}

	patches := make([]*omni.ConfigPatch, 0, list.Len())

	for patch := range list.All() {
		if patch.Metadata().Phase() != resource.PhaseRunning {
			continue
		}

		patches = append(patches, patch)
	}

	slices.SortFunc(patches, func(a, b *omni.ConfigPatch) int {
		return strings.Compare(a.Metadata().ID(), b.Metadata().ID())
	})

	result := make([]string, 0, len(patches))

	for _, patch := range patches {
		buffer, bufErr := patch.TypedSpec().Value.GetUncompressedData()
		if bufErr != nil {
			return nil, bufErr
		}

		result = append(result, string(buffer.Data()))

		buffer.Free()
	}

	return result, nil
}

// desiredConfigHash hashes the inputs that determine the maintenance config, so we only re-apply when they actually change.
func desiredConfigHash(machinePatches []string, baseConfig []byte) string {
	hash := sha256.New()

	for _, patch := range machinePatches {
		hash.Write([]byte(patch))
		hash.Write([]byte{0})
	}

	hash.Write(baseConfig)

	return hex.EncodeToString(hash.Sum(nil))
}

// stripV1Alpha1 removes the v1alpha1 document (if any) from the config, leaving the partial config documents intact.
func stripV1Alpha1(provider config.Provider) (config.Provider, error) {
	documents := provider.Documents()
	filtered := make([]talosconfig.Document, 0, len(documents))
	stripped := false

	for _, document := range documents {
		if document.APIVersion() == "" && document.Kind() == v1alpha1.Version {
			stripped = true

			continue
		}

		filtered = append(filtered, document)
	}

	if !stripped {
		return provider, nil
	}

	return container.New(filtered...)
}
