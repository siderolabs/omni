// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package validations

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	virtualres "github.com/siderolabs/omni/client/pkg/omni/resources/virtual"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/validated"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/virtual/pkg/factory/configs"
)

//nolint:gocognit
func installationMediaConfigValidationOptions(st state.State) []validated.StateOption {
	validateInstallationMedia := func(ctx context.Context, res *omni.InstallationMediaConfig) error {
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

		if err := validateUserString("kernel args", spec.GetKernelArgs(), MaxExtraKernelArgsLength); err != nil {
			return err
		}

		// Strip a leading "v" so older omnictl clients that did not yet normalize the user input
		// still validate against the canonical "1.2.3" form stored on Talos version resources and
		// the extensions catalog.
		talosVersion := strings.TrimPrefix(spec.GetTalosVersion(), "v")

		if talosVersion != "" {
			if _, err := safe.StateGet[*omni.TalosVersion](ctx, st, omni.NewTalosVersion(talosVersion).Metadata()); err != nil {
				if state.IsNotFoundError(err) {
					return fmt.Errorf("unknown Talos version %q", spec.GetTalosVersion())
				}

				return fmt.Errorf("failed to look up Talos version %q: %w", spec.GetTalosVersion(), err)
			}

			// An empty Talos version tracks the server default at download time, which always
			// supports embedded config, so only gate when a concrete version is pinned.
			if spec.GetEmbeddedMachineConfig() != "" && !quirks.New(talosVersion).SupportsEmbeddedConfig() {
				return fmt.Errorf("embedded machine config is not supported by Talos version %q", spec.GetTalosVersion())
			}
		}

		return validateExtensions(ctx, st, talosVersion, spec.GetInstallExtensions())
	}

	return []validated.StateOption{
		validated.WithCreateValidations(validated.NewCreateValidationForType(func(ctx context.Context, res *omni.InstallationMediaConfig, _ ...state.CreateOption) error {
			return validateInstallationMedia(ctx, res)
		})),
		validated.WithUpdateValidations(validated.NewUpdateValidationForType(
			func(ctx context.Context, _, newRes *omni.InstallationMediaConfig, _ ...state.UpdateOption) error {
				return validateInstallationMedia(ctx, newRes)
			},
		)),
	}
}
