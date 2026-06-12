// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package machineconfig

import (
	"context"
	"fmt"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xerrors"
	documentconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	runtimecfg "github.com/siderolabs/talos/pkg/machinery/config/types/runtime"
	talossiderolink "github.com/siderolabs/talos/pkg/machinery/config/types/siderolink"
	configres "github.com/siderolabs/talos/pkg/machinery/resources/config"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/talos"
)

// ExtractionControllerName is the name of the MachineConfigExtraction controller.
const ExtractionControllerName = "MachineConfigExtractionController"

const (
	// configPatchIDPrefix is a low-priority prefix, so that user-created machine config patches (default priority) override this one.
	configPatchIDPrefix = "000-initial-machine-config-"

	configPatchName        = "initial-machine-config"
	configPatchDescription = "Machine configuration the machine connected to Omni with, captured on its first connection."
)

// Reader reads the configuration a machine currently runs, by machine ID.
//
// It works for both clustered and maintenance-mode machines.
type Reader interface {
	ReadMachineConfig(ctx context.Context, machineID string) (*configres.MachineConfig, error)
}

// ExtractionController extracts the config a machine arrives with into a user-managed ConfigPatch, once, while the machine is in maintenance mode.
type ExtractionController struct {
	*qtransform.QController[*omni.MachineStatus, *omni.MachineConfigExtractionStatus]
	machineConfig Reader
}

// NewExtractionController initializes ExtractionController.
func NewExtractionController(machineConfig Reader) *ExtractionController {
	ctrl := &ExtractionController{
		machineConfig: machineConfig,
	}

	ctrl.QController = qtransform.NewQController(
		qtransform.Settings[*omni.MachineStatus, *omni.MachineConfigExtractionStatus]{
			Name: ExtractionControllerName,
			MapMetadataFunc: func(machineStatus *omni.MachineStatus) *omni.MachineConfigExtractionStatus {
				return omni.NewMachineConfigExtractionStatus(machineStatus.Metadata().ID())
			},
			UnmapMetadataFunc: func(status *omni.MachineConfigExtractionStatus) *omni.MachineStatus {
				return omni.NewMachineStatus(status.Metadata().ID())
			},
			TransformExtraOutputFunc: ctrl.transform,
		},
		qtransform.WithExtraOutputs(controller.Output{
			Type: omni.ConfigPatchType,
			Kind: controller.OutputShared,
		}),
		qtransform.WithConcurrency(8),
	)

	return ctrl
}

func (ctrl *ExtractionController) transform(ctx context.Context, r controller.ReaderWriter, _ *zap.Logger, machineStatus *omni.MachineStatus, status *omni.MachineConfigExtractionStatus) error {
	// extract only once: never re-extract, even if the machine reboots with a different config or the user deletes the extracted patch
	if status.TypedSpec().Value.Initialized {
		return nil
	}

	if !machineStatus.TypedSpec().Value.Maintenance {
		return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("machine is not in maintenance mode")
	}

	if machineStatus.TypedSpec().Value.PowerState == specs.MachineStatusSpec_POWER_STATE_OFF {
		return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("machine is powered off")
	}

	if machineStatus.TypedSpec().Value.Schematic.GetInAgentMode() {
		return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("machine is in agent mode, cannot read config")
	}

	// a machine in an invalid state runs its own PKI, so the insecure maintenance API will not let us read its config: skip it.
	if _, invalidState := machineStatus.Metadata().Labels().Get(omni.MachineStatusLabelInvalidState); invalidState {
		return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("machine is in an invalid state, cannot read config")
	}

	if machineStatus.TypedSpec().Value.ManagementAddress == "" {
		return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("machine has no management address yet")
	}

	if !machineStatus.TypedSpec().Value.Connected {
		return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("machine is not connected over siderolink")
	}

	readConfigCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	observedConfig, err := ctrl.machineConfig.ReadMachineConfig(readConfigCtx, machineStatus.Metadata().ID())
	if err != nil {
		return fmt.Errorf("error getting machine config: %w", err)
	}

	var observedConfigBytes []byte

	if observedConfig != nil {
		if observedConfigBytes, err = observedConfig.Container().Bytes(); err != nil {
			return fmt.Errorf("error encoding observed machine config: %w", err)
		}
	}

	// build a user-managed config patch out of whatever the machine arrived with. a machine that arrived with no config
	// (or only Omni-managed documents) gets no patch, but is still marked as initialized below.
	configPatch, reason, err := ctrl.BuildConfigPatch(machineStatus.Metadata().ID(), observedConfigBytes)
	if err != nil {
		return fmt.Errorf("error building machine config patch: %w", err)
	}

	if configPatch != nil {
		// the patch is ownerless on purpose: the user manages it from here on, like any other machine config patch.
		if err = r.Create(ctx, configPatch, controller.WithCreateNoOwner()); err != nil && !state.IsConflictError(err) {
			return fmt.Errorf("error creating machine config patch: %w", err)
		}
	}

	// extraction runs exactly once, so record why the config could not be kept (if so) for the user to see, and stop reconciling.
	status.TypedSpec().Value.Initialized = true
	status.TypedSpec().Value.Error = reason

	return nil
}

// BuildConfigPatch turns the observed machine config into a machine-level ConfigPatch carrying the documents worth keeping.
//
// The Omni-managed connection documents (SideroLink, event sink, kmsg log) are dropped, as Omni regenerates them. The rest
// is kept so it survives the machine lifecycle.
//
// It returns a nil patch and an empty reason when there is nothing to keep. It returns a nil patch and a non-empty reason
// when the kept documents do not form a valid config patch: the patch is created bypassing the regular config patch
// validation, so we apply the same rules here to never surface a patch the user could not have created or edited.
func (ctrl *ExtractionController) BuildConfigPatch(machineID string, observedConfig []byte) (*omni.ConfigPatch, string, error) {
	if len(observedConfig) == 0 {
		return nil, "", nil
	}

	provider, err := configloader.NewFromBytes(observedConfig)
	if err != nil {
		return nil, "", fmt.Errorf("failed to load observed machine config: %w", err)
	}

	documents := provider.Documents()
	kept := make([]documentconfig.Document, 0, len(documents))

	for _, document := range documents {
		if ctrl.isConnectionDocument(document) {
			continue
		}

		kept = append(kept, document)
	}

	if len(kept) == 0 {
		return nil, "", nil
	}

	keptContainer, err := container.New(kept...)
	if err != nil {
		return nil, "", fmt.Errorf("failed to build config container: %w", err)
	}

	data, err := keptContainer.EncodeBytes(encoder.WithComments(encoder.CommentsDisabled))
	if err != nil {
		return nil, "", fmt.Errorf("failed to encode config: %w", err)
	}

	if validationErr := omni.ValidateConfigPatch(data); validationErr != nil {
		return nil, fmt.Sprintf("incoming machine config cannot be kept as a config patch: %s", validationErr), nil
	}

	configPatch := omni.NewConfigPatch(configPatchIDPrefix + machineID)

	if err = configPatch.TypedSpec().Value.SetUncompressedData(data); err != nil {
		return nil, "", fmt.Errorf("failed to set config patch data: %w", err)
	}

	configPatch.Metadata().Labels().Set(omni.LabelMachine, machineID)
	configPatch.Metadata().Annotations().Set(omni.ConfigPatchName, configPatchName)
	configPatch.Metadata().Annotations().Set(omni.ConfigPatchDescription, configPatchDescription)

	return configPatch, "", nil
}

// isConnectionDocument reports whether the document is one of the Omni-managed connection documents that Omni regenerates itself.
func (ctrl *ExtractionController) isConnectionDocument(document documentconfig.Document) bool {
	switch document.Kind() {
	case talossiderolink.Kind, runtimecfg.EventSinkKind, runtimecfg.KmsgLogKind:
		return true
	default:
		return false
	}
}

// NewReader returns a Reader backed by the cached Talos client factory.
//
// It reads the machine config using a maintenance mode Talos client.
func NewReader(clientFactory *talos.ClientFactory) Reader {
	return reader{clientFactory: clientFactory}
}

type reader struct {
	clientFactory *talos.ClientFactory
}

func (r reader) ReadMachineConfig(ctx context.Context, machineID string) (*configres.MachineConfig, error) {
	client, err := r.clientFactory.GetMaintenance(ctx, machineID)
	if err != nil {
		return nil, err
	}

	machineConfig, err := safe.ReaderGetByID[*configres.MachineConfig](ctx, client.COSI, configres.ActiveID)
	if err != nil && !state.IsNotFoundError(err) {
		return nil, err
	}

	return machineConfig, nil
}
