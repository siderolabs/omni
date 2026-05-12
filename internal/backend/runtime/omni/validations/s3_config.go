// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package validations

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/etcdbackup/store"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/validated"
)

func s3ConfigValidationOptions() []validated.StateOption {
	return []validated.StateOption{
		validated.WithCreateValidations(validated.NewCreateValidationForType(func(ctx context.Context, res *omni.EtcdBackupS3Conf, _ ...state.CreateOption) error {
			return validateS3Configuration(ctx, res)
		})),
		validated.WithUpdateValidations(validated.NewUpdateValidationForType(func(ctx context.Context, _ *omni.EtcdBackupS3Conf, newRes *omni.EtcdBackupS3Conf, _ ...state.UpdateOption) error {
			return validateS3Configuration(ctx, newRes)
		})),
	}
}

func validateS3Configuration(ctx context.Context, s3Conf *omni.EtcdBackupS3Conf) error {
	if store.IsEmptyS3Conf(s3Conf) {
		return nil
	}

	_, _, err := store.S3ClientFromResource(ctx, s3Conf)
	if err != nil {
		return fmt.Errorf("incorrect settings for s3 client: %w", err)
	}

	return nil
}
