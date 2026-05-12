// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package validations

import (
	"context"
	"errors"
	"time"

	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/validated"
)

func etcdManualBackupValidationOptions() []validated.StateOption {
	return []validated.StateOption{
		validated.WithCreateValidations(validated.NewCreateValidationForType(func(_ context.Context, res *omni.EtcdManualBackup, _ ...state.CreateOption) error {
			return validateManualBackup(res.TypedSpec())
		})),
		validated.WithUpdateValidations(validated.NewUpdateValidationForType(func(_ context.Context, oldRes *omni.EtcdManualBackup, newRes *omni.EtcdManualBackup, _ ...state.UpdateOption) error {
			if oldRes == nil {
				return nil
			}

			if oldRes.TypedSpec().Value.BackupAt.AsTime().Equal(newRes.TypedSpec().Value.BackupAt.AsTime()) {
				return nil
			}

			return validateManualBackup(newRes.TypedSpec())
		})),
	}
}

func validateManualBackup(embs *omni.EtcdManualBackupSpec) error {
	backupAt := embs.Value.GetBackupAt().AsTime()

	if time.Since(backupAt) > time.Minute {
		return errors.New("backup time must not be more than 1 minute in the past")
	} else if time.Until(backupAt) > time.Minute {
		return errors.New("backup time must not be more than 1 minute in the future")
	}

	return nil
}
