// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package download_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/virtual"
	"github.com/siderolabs/omni/client/pkg/omnictl/internal/download"
)

func newTestState(t *testing.T) state.State {
	t.Helper()

	return state.WrapCore(namespaced.NewState(inmem.Build))
}

func TestValidateCloudPlatform(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	cfg := virtual.NewCloudPlatformConfig("aws")
	cfg.TypedSpec().Value.Architectures = []specs.PlatformConfigSpec_Arch{
		specs.PlatformConfigSpec_AMD64,
		specs.PlatformConfigSpec_ARM64,
	}
	cfg.TypedSpec().Value.SecureBootSupported = false
	cfg.TypedSpec().Value.MinVersion = "1.10.0"

	st := newTestState(t)
	require.NoError(t, st.Create(ctx, cfg))

	t.Run("happy path", func(t *testing.T) {
		t.Parallel()

		require.NoError(t, download.ValidateCloudPlatform(ctx, st, "aws", specs.PlatformConfigSpec_AMD64, false, "1.13.0"))
	})

	t.Run("unknown platform", func(t *testing.T) {
		t.Parallel()

		err := download.ValidateCloudPlatform(ctx, st, "azure", specs.PlatformConfigSpec_AMD64, false, "1.13.0")
		require.Error(t, err)
		require.Contains(t, err.Error(), `failed to get cloud platform config for "azure"`)
	})

	t.Run("unsupported arch", func(t *testing.T) {
		t.Parallel()

		amdOnly := virtual.NewCloudPlatformConfig("amd-only")
		amdOnly.TypedSpec().Value.Architectures = []specs.PlatformConfigSpec_Arch{specs.PlatformConfigSpec_AMD64}
		require.NoError(t, st.Create(ctx, amdOnly))

		err := download.ValidateCloudPlatform(ctx, st, "amd-only", specs.PlatformConfigSpec_ARM64, false, "1.13.0")
		require.Error(t, err)
		require.Contains(t, err.Error(), `cloud platform "amd-only" does not support architecture "arm64"`)
	})

	t.Run("secure boot unsupported", func(t *testing.T) {
		t.Parallel()

		err := download.ValidateCloudPlatform(ctx, st, "aws", specs.PlatformConfigSpec_AMD64, true, "1.13.0")
		require.Error(t, err)
		require.Contains(t, err.Error(), `cloud platform "aws" does not support secure boot`)
	})

	t.Run("talos version below min", func(t *testing.T) {
		t.Parallel()

		err := download.ValidateCloudPlatform(ctx, st, "aws", specs.PlatformConfigSpec_AMD64, false, "1.9.0")
		require.Error(t, err)
		require.Contains(t, err.Error(), `cloud platform "aws" requires Talos version >= 1.10.0`)
	})

	t.Run("min version unset", func(t *testing.T) {
		t.Parallel()

		noMin := virtual.NewCloudPlatformConfig("no-min")
		noMin.TypedSpec().Value.Architectures = []specs.PlatformConfigSpec_Arch{specs.PlatformConfigSpec_AMD64}
		require.NoError(t, st.Create(ctx, noMin))

		require.NoError(t, download.ValidateCloudPlatform(ctx, st, "no-min", specs.PlatformConfigSpec_AMD64, false, "1.0.0"))
	})
}

func TestValidateSBC(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	cfg := virtual.NewSBCConfig("rpi_generic")
	cfg.TypedSpec().Value.MinVersion = "1.11.0"

	st := newTestState(t)
	require.NoError(t, st.Create(ctx, cfg))

	t.Run("happy path", func(t *testing.T) {
		t.Parallel()

		require.NoError(t, download.ValidateSBC(ctx, st, "rpi_generic", "1.13.0"))
	})

	t.Run("unknown overlay", func(t *testing.T) {
		t.Parallel()

		err := download.ValidateSBC(ctx, st, "nonexistent", "1.13.0")
		require.Error(t, err)
		require.Contains(t, err.Error(), `failed to get SBC config for overlay "nonexistent"`)
	})

	t.Run("talos version below min", func(t *testing.T) {
		t.Parallel()

		err := download.ValidateSBC(ctx, st, "rpi_generic", "1.10.0")
		require.Error(t, err)
		require.Contains(t, err.Error(), `SBC overlay "rpi_generic" requires Talos version >= 1.11.0`)
	})
}

func TestValidateTalosVersion(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	st := newTestState(t)

	known := omni.NewTalosVersion("1.13.0")
	require.NoError(t, st.Create(ctx, known))

	t.Run("known version", func(t *testing.T) {
		t.Parallel()

		require.NoError(t, download.ValidateTalosVersion(ctx, st, "1.13.0"))
	})

	t.Run("v-prefix accepted", func(t *testing.T) {
		t.Parallel()

		require.NoError(t, download.ValidateTalosVersion(ctx, st, "v1.13.0"))
	})

	t.Run("unknown version is rejected", func(t *testing.T) {
		t.Parallel()

		err := download.ValidateTalosVersion(ctx, st, "9.99.99")
		require.Error(t, err)
		require.Contains(t, err.Error(), `unknown Talos version "9.99.99"`)
	})
}

func TestValidateExtensions(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	ext := omni.NewTalosExtensions("1.13.0")
	ext.TypedSpec().Value.Items = []*specs.TalosExtensionsSpec_Info{
		{Name: "siderolabs/qemu-guest-agent"},
		{Name: "siderolabs/intel-ucode"},
		{Name: "siderolabs/amd-ucode"},
	}

	st := newTestState(t)
	require.NoError(t, st.Create(ctx, ext))

	t.Run("empty list is allowed", func(t *testing.T) {
		t.Parallel()

		require.NoError(t, download.ValidateExtensions(ctx, st, "1.13.0", nil))
	})

	t.Run("happy path", func(t *testing.T) {
		t.Parallel()

		require.NoError(t, download.ValidateExtensions(ctx, st, "1.13.0", []string{"qemu-guest-agent", "intel-ucode"}))
	})

	t.Run("unknown talos version", func(t *testing.T) {
		t.Parallel()

		err := download.ValidateExtensions(ctx, st, "9.99.99", []string{"qemu-guest-agent"})
		require.Error(t, err)
		require.Contains(t, err.Error(), `failed to get extensions for talos version "9.99.99"`)
	})

	t.Run("unknown extension", func(t *testing.T) {
		t.Parallel()

		err := download.ValidateExtensions(ctx, st, "1.13.0", []string{"qemu-guest-agent", "nonexistent"})
		require.Error(t, err)
		require.Contains(t, err.Error(), `failed to find extension with name "nonexistent" for talos version "1.13.0"`)
	})
}

func TestParseArchRoundTrip(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"amd64", "arm64", "AMD64", "ARM64"} {
		got, err := download.ParseArch(name)
		require.NoError(t, err)
		require.Equal(t, strings.ToLower(name), download.ArchToString(got))
	}

	_, err := download.ParseArch("riscv64")
	require.Error(t, err)
}

func TestBootloaderRoundTrip(t *testing.T) {
	t.Parallel()

	for _, in := range []string{download.BootloaderUEFI, download.BootloaderBIOS, download.BootloaderDual, download.BootloaderAuto} {
		got, err := download.ParseBootloader(in)
		require.NoError(t, err)
		require.Equal(t, in, download.BootloaderToString(got))
	}

	_, err := download.ParseBootloader("incorrect")
	require.Error(t, err)
	require.Contains(t, err.Error(), `unknown bootloader "incorrect"`)
}

func TestBuildParamsFromPresetDefaults(t *testing.T) {
	t.Parallel()

	t.Run("empty TalosVersion falls back to default", func(t *testing.T) {
		t.Parallel()

		spec := &specs.InstallationMediaConfigSpec{}
		params, err := download.BuildParamsFromPreset(spec, "amd64")
		require.NoError(t, err)
		require.Equal(t, constants.DefaultTalosVersion, params.TalosVersion)
	})

	t.Run("explicit TalosVersion is preserved", func(t *testing.T) {
		t.Parallel()

		spec := &specs.InstallationMediaConfigSpec{TalosVersion: "1.10.0"}
		params, err := download.BuildParamsFromPreset(spec, "amd64")
		require.NoError(t, err)
		require.Equal(t, "1.10.0", params.TalosVersion)
	})

	t.Run("empty JoinToken is left empty for ResolveJoinToken to handle", func(t *testing.T) {
		t.Parallel()

		spec := &specs.InstallationMediaConfigSpec{}
		params, err := download.BuildParamsFromPreset(spec, "amd64")
		require.NoError(t, err)
		require.Empty(t, params.JoinToken)
	})

	t.Run("EmbeddedMachineConfig is carried over", func(t *testing.T) {
		t.Parallel()

		spec := &specs.InstallationMediaConfigSpec{EmbeddedMachineConfig: "version: v1alpha1\nmachine: {}\n"}
		params, err := download.BuildParamsFromPreset(spec, "amd64")
		require.NoError(t, err)
		require.Equal(t, "version: v1alpha1\nmachine: {}\n", params.EmbeddedMachineConfig)
	})
}

func TestReadEmbeddedMachineConfigFile(t *testing.T) {
	t.Parallel()

	t.Run("returns empty when no path is given", func(t *testing.T) {
		t.Parallel()

		out, err := download.ReadEmbeddedMachineConfigFile("")
		require.NoError(t, err)
		require.Empty(t, out)
	})

	t.Run("reads the file when a path is given", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), "config.yaml")
		require.NoError(t, os.WriteFile(path, []byte("version: v1alpha1\nmachine: {}\n"), 0o600))

		out, err := download.ReadEmbeddedMachineConfigFile(path)
		require.NoError(t, err)
		require.Equal(t, "version: v1alpha1\nmachine: {}\n", out)
	})

	t.Run("errors with the path when the file is missing", func(t *testing.T) {
		t.Parallel()

		_, err := download.ReadEmbeddedMachineConfigFile(filepath.Join(t.TempDir(), "missing.yaml"))
		require.Error(t, err)
		require.Contains(t, err.Error(), "missing.yaml")
	})
}

func TestValidateEmbeddedConfigSupport(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	newQuirks := func(version string, supported bool) *virtual.Quirks {
		q := virtual.NewQuirks(version)
		q.TypedSpec().Value.SupportsEmbeddedConfig = supported

		return q
	}

	t.Run("passes when the version supports embedded config", func(t *testing.T) {
		t.Parallel()

		st := newTestState(t)
		require.NoError(t, st.Create(ctx, newQuirks("1.13.0", true)))

		require.NoError(t, download.ValidateEmbeddedConfigSupport(ctx, st, "1.13.0"))
	})

	t.Run("fails when the version does not support embedded config", func(t *testing.T) {
		t.Parallel()

		st := newTestState(t)
		require.NoError(t, st.Create(ctx, newQuirks("1.11.0", false)))

		err := download.ValidateEmbeddedConfigSupport(ctx, st, "1.11.0")
		require.Error(t, err)
		require.Contains(t, err.Error(), "embedded machine config is not supported by Talos")
	})

	t.Run("strips a leading v before the lookup", func(t *testing.T) {
		t.Parallel()

		st := newTestState(t)
		require.NoError(t, st.Create(ctx, newQuirks("1.13.0", true)))

		require.NoError(t, download.ValidateEmbeddedConfigSupport(ctx, st, "v1.13.0"))
	})
}

func TestImageFactoryCredentials(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	newFeatures := func(enterprise bool) *omni.FeaturesConfig {
		features := omni.NewFeaturesConfig(omni.FeaturesConfigID)
		features.TypedSpec().Value.IsEnterpriseImageFactory = enterprise

		return features
	}

	t.Run("public factory returns no credentials without reading the auth resource", func(t *testing.T) {
		t.Parallel()

		// State is intentionally empty: a public factory must not require the auth resource.
		st := newTestState(t)

		username, password, err := download.ImageFactoryCredentials(ctx, st, newFeatures(false))
		require.NoError(t, err)
		require.Empty(t, username)
		require.Empty(t, password)
	})

	t.Run("enterprise factory returns the stored credentials", func(t *testing.T) {
		t.Parallel()

		st := newTestState(t)

		auth := virtual.NewImageFactoryAuth()
		auth.TypedSpec().Value.Username = "omni-e2e-test"
		auth.TypedSpec().Value.Password = "s3cr3t"
		require.NoError(t, st.Create(ctx, auth))

		username, password, err := download.ImageFactoryCredentials(ctx, st, newFeatures(true))
		require.NoError(t, err)
		require.Equal(t, "omni-e2e-test", username)
		require.Equal(t, "s3cr3t", password)
	})

	t.Run("enterprise factory errors when the auth resource is missing", func(t *testing.T) {
		t.Parallel()

		st := newTestState(t)

		_, _, err := download.ImageFactoryCredentials(ctx, st, newFeatures(true))
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to get image factory auth")
	})
}

func TestGrpcTunnelModeToString(t *testing.T) {
	t.Parallel()

	require.Equal(t, download.GrpcTunnelEnabled, download.GrpcTunnelModeToString(specs.GrpcTunnelMode_ENABLED))
	require.Equal(t, download.GrpcTunnelDisabled, download.GrpcTunnelModeToString(specs.GrpcTunnelMode_DISABLED))
	require.Equal(t, download.GrpcTunnelAuto, download.GrpcTunnelModeToString(specs.GrpcTunnelMode_UNSET))
}
