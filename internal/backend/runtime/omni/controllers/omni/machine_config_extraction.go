// Copyright (c) 2026 Sidero Labs, Inc.
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
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/siderolabs/gen/xerrors"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// MachineConfigExtractor extracts the config a machine arrived with into a user-managed ConfigPatch.
//
// It returns a non-empty reason (and creates no patch) when the incoming config cannot be preserved as a user-editable config patch.
type MachineConfigExtractor interface {
	Extract(ctx context.Context, id resource.ID, observedConfig []byte) (string, error)
}

// MachineConfigExtractionController extracts the config a machine arrives with into a user-managed ConfigPatch, once, while the machine is in maintenance mode.
type MachineConfigExtractionController = qtransform.QController[*omni.MachineStatus, *omni.MachineConfigExtractionStatus]

// NewMachineConfigExtractionController initializes MachineConfigExtractionController.
func NewMachineConfigExtractionController(maintenanceClientFactory MaintenanceClientFactory, extractor MachineConfigExtractor) *MachineConfigExtractionController {
	if maintenanceClientFactory == nil {
		maintenanceClientFactory = DefaultMaintenanceClientFactory
	}

	return qtransform.NewQController(
		qtransform.Settings[*omni.MachineStatus, *omni.MachineConfigExtractionStatus]{
			Name: "MachineConfigExtractionController",
			MapMetadataFunc: func(machineStatus *omni.MachineStatus) *omni.MachineConfigExtractionStatus {
				return omni.NewMachineConfigExtractionStatus(machineStatus.Metadata().ID())
			},
			UnmapMetadataFunc: func(status *omni.MachineConfigExtractionStatus) *omni.MachineStatus {
				return omni.NewMachineStatus(status.Metadata().ID())
			},
			TransformFunc: func(ctx context.Context, _ controller.Reader, _ *zap.Logger, machineStatus *omni.MachineStatus, status *omni.MachineConfigExtractionStatus) error {
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

				managementAddress := machineStatus.TypedSpec().Value.ManagementAddress
				if managementAddress == "" {
					return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("machine has no management address yet")
				}

				maintenanceClient, err := maintenanceClientFactory(ctx, managementAddress)
				if err != nil {
					return fmt.Errorf("error creating maintenance client: %w", err)
				}

				getConfigCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
				defer cancel()

				observedConfig, err := maintenanceClient.GetMachineConfig(getConfigCtx)
				if err != nil {
					return fmt.Errorf("error getting machine config: %w", err)
				}

				var observedConfigBytes []byte

				if observedConfig != nil {
					if observedConfigBytes, err = observedConfig.Provider().Bytes(); err != nil {
						return fmt.Errorf("error encoding observed machine config: %w", err)
					}
				}

				// extract whatever the machine arrived with into a user-managed config patch. a machine that arrived with no config
				// (or only Omni-managed documents) gets no patch, but is still marked as initialized below.
				reason, err := extractor.Extract(ctx, machineStatus.Metadata().ID(), observedConfigBytes)
				if err != nil {
					return fmt.Errorf("error extracting machine config: %w", err)
				}

				// extraction runs exactly once, so record why the config could not be preserved (if so) for the user to see, and stop reconciling.
				status.TypedSpec().Value.Initialized = true
				status.TypedSpec().Value.Error = reason

				return nil
			},
		},
		qtransform.WithConcurrency(32),
	)
}
