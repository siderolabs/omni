// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package validations

import (
	"context"
	"errors"
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	virtualres "github.com/siderolabs/omni/client/pkg/omni/resources/virtual"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/validated"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/virtual/pkg/factory/configs"
)

func installationMediaConfigValidationOptions() []validated.StateOption {
	validateInstallationMedia := func(res *omni.InstallationMediaConfig) error {
		if res.Metadata().Phase() == resource.PhaseTearingDown {
			return nil
		}

		spec := res.TypedSpec().Value

		if spec.Architecture == specs.PlatformConfigSpec_UNKNOWN_ARCH {
			return errors.New("invalid installation media config: architecture is required")
		}

		if spec.Cloud != nil && spec.Sbc != nil {
			return errors.New("invalid installation media config: both sbc and cloud fields are set")
		}

		if spec.Sbc != nil {
			if spec.Sbc.Overlay == "" {
				return errors.New("invalid installation media config: sbc.overlay is required when sbc is set")
			}

			if _, err := configs.GetSBCConfig(virtualres.NewSBCConfig(spec.Sbc.Overlay).Metadata()); err != nil {
				return fmt.Errorf("invalid installation media config: unknown SBC overlay %q", spec.Sbc.Overlay)
			}
		}

		if spec.Cloud != nil {
			if spec.Cloud.Platform == "" {
				return errors.New("invalid installation media config: cloud.platform is required when cloud is set")
			}

			if _, err := configs.GetCloudPlatformConfig(virtualres.NewCloudPlatformConfig(spec.Cloud.Platform).Metadata()); err != nil {
				return fmt.Errorf("invalid installation media config: unknown cloud platform %q", spec.Cloud.Platform)
			}
		}

		return nil
	}

	return []validated.StateOption{
		validated.WithCreateValidations(validated.NewCreateValidationForType(func(ctx context.Context, res *omni.InstallationMediaConfig, _ ...state.CreateOption) error {
			return validateInstallationMedia(res)
		})),
		validated.WithUpdateValidations(validated.NewUpdateValidationForType(
			func(ctx context.Context, _, newRes *omni.InstallationMediaConfig, _ ...state.UpdateOption) error {
				return validateInstallationMedia(newRes)
			},
		)),
	}
}
