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
	"github.com/siderolabs/omni/client/pkg/imagefactory"
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

func TestMediaSpec(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		image    download.ImageInfo
		params   download.Params
		expected imagefactory.MediaSpec
	}{
		{
			name:     "metal ISO",
			image:    download.ImageInfo{Profile: "metal", Architecture: "amd64", Extension: "iso"},
			expected: imagefactory.MediaSpec{Kind: imagefactory.InstallationMediaKindISO, Platform: "metal", Architecture: "amd64", Format: "iso"},
		},
		{
			name:     "metal raw disk image",
			image:    download.ImageInfo{Profile: "metal", Architecture: "arm64", Extension: "raw.xz"},
			expected: imagefactory.MediaSpec{Kind: imagefactory.InstallationMediaKindDisk, Platform: "metal", Architecture: "arm64", Format: "raw.xz"},
		},
		{
			name:     "metal qcow2 disk image",
			image:    download.ImageInfo{Profile: "metal", Architecture: "amd64", Extension: "qcow2"},
			expected: imagefactory.MediaSpec{Kind: imagefactory.InstallationMediaKindDisk, Platform: "metal", Architecture: "amd64", Format: "qcow2"},
		},
		{
			name:     "cloud platform",
			image:    download.ImageInfo{Profile: "aws", Architecture: "amd64", Extension: "raw.xz"},
			expected: imagefactory.MediaSpec{Kind: imagefactory.InstallationMediaKindDisk, Platform: "aws", Architecture: "amd64", Format: "raw.xz"},
		},
		{
			name:     "secure boot ISO",
			image:    download.ImageInfo{Profile: "metal", Architecture: "amd64", Extension: "iso"},
			params:   download.Params{SecureBoot: true},
			expected: imagefactory.MediaSpec{Kind: imagefactory.InstallationMediaKindISO, Platform: "metal", Architecture: "amd64", Format: "iso", SecureBoot: true},
		},
		{
			// PXE wins over the extension, which a preset leaves at "iso" because nothing is downloaded.
			name:     "PXE",
			image:    download.ImageInfo{Profile: "metal", Architecture: "amd64", Extension: "iso"},
			params:   download.Params{PXE: true},
			expected: imagefactory.MediaSpec{Kind: imagefactory.InstallationMediaKindPXE, Platform: "metal", Architecture: "amd64", Format: "iso"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			spec := download.MediaSpec(test.image, test.params)

			require.Equal(t, test.expected, spec)
			require.NoError(t, spec.Validate())
		})
	}
}

func TestGrpcTunnelModeToString(t *testing.T) {
	t.Parallel()

	require.Equal(t, download.GrpcTunnelEnabled, download.GrpcTunnelModeToString(specs.GrpcTunnelMode_ENABLED))
	require.Equal(t, download.GrpcTunnelDisabled, download.GrpcTunnelModeToString(specs.GrpcTunnelMode_DISABLED))
	require.Equal(t, download.GrpcTunnelAuto, download.GrpcTunnelModeToString(specs.GrpcTunnelMode_UNSET))
}
