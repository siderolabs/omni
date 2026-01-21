// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// EtcdBackupOverallStatusController is a controller that manages EtcdBackupOverallStatus.
type EtcdBackupOverallStatusController struct{}

// NewEtcdBackupOverallStatusController initializes EtcdBackupOverallStatusController.
func NewEtcdBackupOverallStatusController() *EtcdBackupOverallStatusController {
	return &EtcdBackupOverallStatusController{}
}

// Name implements controller.Controller interface.
func (ctrl *EtcdBackupOverallStatusController) Name() string {
	return "EtcdBackupOverallStatusController"
}

// Inputs implements controller.Controller interface.
func (ctrl *EtcdBackupOverallStatusController) Inputs() []controller.Input {
	return []controller.Input{
		safe.Input[*omni.EtcdBackupStoreStatus](controller.InputWeak),
		safe.Input[*omni.EtcdBackupStatus](controller.InputWeak),
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *EtcdBackupOverallStatusController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: omni.EtcdBackupOverallStatusType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *EtcdBackupOverallStatusController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		storeStatus, err := safe.ReaderGetByID[*omni.EtcdBackupStoreStatus](ctx, r, omni.EtcdBackupStoreStatusID)
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("failed to get etcd backup store status: %w", err)
		}

		backupStatusList, err := safe.ReaderListAll[*omni.EtcdBackupStatus](ctx, r)
		if err != nil {
			return fmt.Errorf("failed to list etcd backup statuses: %w", err)
		}

		var latestStatus *omni.EtcdBackupStatus

		backupStatusList.ForEach(func(status *omni.EtcdBackupStatus) {
			if latestStatus == nil || status.Metadata().Updated().After(latestStatus.Metadata().Updated()) {
				latestStatus = status
			}
		})

		err = safe.WriterModify(ctx, r, omni.NewEtcdBackupOverallStatus(), func(output *omni.EtcdBackupOverallStatus) error {
			output.TypedSpec().Value.ConfigurationName = storeStatus.TypedSpec().Value.ConfigurationName
			output.TypedSpec().Value.ConfigurationError = storeStatus.TypedSpec().Value.ConfigurationError

			if latestStatus != nil {
				if output.TypedSpec().Value.LastBackupStatus == nil {
					output.TypedSpec().Value.LastBackupStatus = &specs.EtcdBackupStatusSpec{}
				}

				output.TypedSpec().Value.LastBackupStatus.Status = latestStatus.TypedSpec().Value.Status
				output.TypedSpec().Value.LastBackupStatus.Error = latestStatus.TypedSpec().Value.Error
				output.TypedSpec().Value.LastBackupStatus.LastBackupTime = latestStatus.TypedSpec().Value.LastBackupTime
				output.TypedSpec().Value.LastBackupStatus.LastBackupAttempt = latestStatus.TypedSpec().Value.LastBackupAttempt
			}

			return nil
		})
		if err != nil {
			return fmt.Errorf("failed to modify etcd backup overall status: %w", err)
		}
	}
}
