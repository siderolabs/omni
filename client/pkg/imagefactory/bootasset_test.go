// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package imagefactory_test

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/client/pkg/imagefactory"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

const (
	primaryURL      = "https://factory.example.org"
	primaryPXEURL   = "https://pxe.factory.example.org"
	secondaryURL    = "https://secondary.example.org"
	secondaryPXEURL = "https://pxe.secondary.example.org"

	schematicID = "376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba"
)

// diskSpec is the asset most providers ask for.
func diskSpec() imagefactory.AssetSpec {
	return imagefactory.AssetSpec{
		Kind:         imagefactory.BootAssetKindDisk,
		Platform:     "nocloud",
		Architecture: "amd64",
		Format:       "raw.xz",
	}
}

func pxeSpec() imagefactory.AssetSpec {
	return imagefactory.AssetSpec{
		Kind:         imagefactory.BootAssetKindPXE,
		Platform:     "metal",
		Architecture: "amd64",
	}
}

func createFeaturesConfig(ctx context.Context, t *testing.T, st state.State, modify func(*omni.FeaturesConfig)) {
	t.Helper()

	config := omni.NewFeaturesConfig(omni.FeaturesConfigID)

	config.TypedSpec().Value.ImageFactoryBaseUrl = primaryURL
	config.TypedSpec().Value.ImageFactoryPxeBaseUrl = primaryPXEURL

	if modify != nil {
		modify(config)
	}

	require.NoError(t, st.Create(ctx, config))
}

func createTalosVersion(ctx context.Context, t *testing.T, st state.State, id, factoryURL string) {
	t.Helper()

	version := omni.NewTalosVersion(id)
	version.TypedSpec().Value.ImageFactoryUrl = factoryURL

	require.NoError(t, st.Create(ctx, version))
}

func createFactoryAuth(ctx context.Context, t *testing.T, st state.State, factoryURL, username, password string) {
	t.Helper()

	auth := omni.NewImageFactoryAuth(factoryURL)
	auth.TypedSpec().Value.Username = username
	auth.TypedSpec().Value.Password = password

	require.NoError(t, st.Create(ctx, auth))
}

// deniedAuthState is a state.State which denies reads of ImageFactoryAuth and serves everything else, the
// way an Omni without the resource or without the role grant does.
type deniedAuthState struct {
	state.State
}

func (s deniedAuthState) Get(ctx context.Context, ptr resource.Pointer, opts ...state.GetOption) (resource.Resource, error) {
	if ptr.Type() == omni.ImageFactoryAuthType {
		return nil, status.Error(codes.PermissionDenied, "no access is permitted")
	}

	return s.State.Get(ctx, ptr, opts...)
}

// TestAssetSpecFilename pins the filename of every asset kind the image factory serves, in the forms
// its own path parser accepts.
func TestAssetSpecFilename(t *testing.T) {
	t.Parallel()

	for name, tt := range map[string]struct {
		expected string
		spec     imagefactory.AssetSpec
	}{
		"pxe": {
			spec:     imagefactory.AssetSpec{Kind: imagefactory.BootAssetKindPXE, Platform: "metal", Architecture: "amd64"},
			expected: "metal-amd64",
		},
		"pxe secure boot": {
			spec:     imagefactory.AssetSpec{Kind: imagefactory.BootAssetKindPXE, Platform: "metal", Architecture: "amd64", SecureBoot: true},
			expected: "metal-amd64-secureboot",
		},
		"iso": {
			spec:     imagefactory.AssetSpec{Kind: imagefactory.BootAssetKindISO, Platform: "metal", Architecture: "arm64"},
			expected: "metal-arm64.iso",
		},
		"iso secure boot": {
			spec:     imagefactory.AssetSpec{Kind: imagefactory.BootAssetKindISO, Platform: "metal", Architecture: "amd64", SecureBoot: true},
			expected: "metal-amd64-secureboot.iso",
		},
		"disk raw xz": {
			spec:     imagefactory.AssetSpec{Kind: imagefactory.BootAssetKindDisk, Platform: "nocloud", Architecture: "amd64", Format: "raw.xz"},
			expected: "nocloud-amd64.raw.xz",
		},
		"disk qcow2": {
			spec:     imagefactory.AssetSpec{Kind: imagefactory.BootAssetKindDisk, Platform: "nocloud", Architecture: "amd64", Format: "qcow2"},
			expected: "nocloud-amd64.qcow2",
		},
		"disk compressed with secure boot": {
			spec:     imagefactory.AssetSpec{Kind: imagefactory.BootAssetKindDisk, Platform: "nocloud", Architecture: "amd64", Format: "qcow2.gz", SecureBoot: true},
			expected: "nocloud-amd64-secureboot.qcow2.gz",
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			require.NoError(t, tt.spec.Validate())
			require.Equal(t, tt.expected, tt.spec.Filename())
		})
	}
}

// TestAssetSpecValidate covers what must not reach the factory as a URL path, and the per-kind
// requirements. The traversal cases are the ones that matter: these fields become path segments and
// generally come from a provider's own configuration.
func TestAssetSpecValidate(t *testing.T) {
	t.Parallel()

	for name, tt := range map[string]struct {
		errContains string
		spec        imagefactory.AssetSpec
	}{
		"kind unset": {
			spec:        imagefactory.AssetSpec{Platform: "metal", Architecture: "amd64"},
			errContains: "boot asset kind is not set",
		},
		"unknown kind": {
			spec:        imagefactory.AssetSpec{Kind: "raw", Platform: "metal", Architecture: "amd64"},
			errContains: `unknown boot asset kind "raw"`,
		},
		"missing architecture": {
			spec:        imagefactory.AssetSpec{Kind: imagefactory.BootAssetKindISO, Platform: "metal"},
			errContains: "invalid architecture",
		},
		"missing platform": {
			spec:        imagefactory.AssetSpec{Kind: imagefactory.BootAssetKindISO, Architecture: "amd64"},
			errContains: "invalid platform",
		},
		"platform traversal": {
			spec:        imagefactory.AssetSpec{Kind: imagefactory.BootAssetKindISO, Platform: "../../secret", Architecture: "amd64"},
			errContains: "invalid platform",
		},
		"architecture with a slash": {
			spec:        imagefactory.AssetSpec{Kind: imagefactory.BootAssetKindISO, Platform: "metal", Architecture: "amd64/../.."},
			errContains: "invalid architecture",
		},
		"format traversal": {
			spec:        imagefactory.AssetSpec{Kind: imagefactory.BootAssetKindDisk, Platform: "nocloud", Architecture: "amd64", Format: "../../etc/passwd"},
			errContains: "invalid disk image format",
		},
		"disk without a format": {
			spec:        imagefactory.AssetSpec{Kind: imagefactory.BootAssetKindDisk, Platform: "nocloud", Architecture: "amd64"},
			errContains: "invalid disk image format",
		},
		"parent directory as a platform": {
			spec:        imagefactory.AssetSpec{Kind: imagefactory.BootAssetKindISO, Platform: "..", Architecture: "amd64"},
			errContains: "invalid platform",
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			require.ErrorContains(t, tt.spec.Validate(), tt.errContains)
		})
	}
}

// TestResolveBootAssetRouting pins which factory serves each Talos version, and that the version
// segment lands in the URL in the factory's canonical v-prefixed form regardless of how the caller
// spells it. The PXE kind goes to the PXE endpoint, everything else to the image endpoint.
func TestResolveBootAssetRouting(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	st := newTestState(t)

	createFeaturesConfig(ctx, t, st, func(config *omni.FeaturesConfig) {
		// configured with a trailing slash, to check that the comparison against the recorded URL normalizes
		config.TypedSpec().Value.SecondaryImageFactoryBaseUrl = secondaryURL + "/"
		config.TypedSpec().Value.SecondaryImageFactoryPxeBaseUrl = secondaryPXEURL
	})

	createTalosVersion(ctx, t, st, "1.13.0", secondaryURL)
	createTalosVersion(ctx, t, st, "1.14.0", primaryURL)
	createTalosVersion(ctx, t, st, "1.12.0", "")

	// Recorded explicitly so the empty-version case asserts the fallback rather than depending on
	// whatever DefaultTalosVersion happens to be unrecorded.
	createTalosVersion(ctx, t, st, constants.DefaultTalosVersion, primaryURL)

	for _, tt := range []struct {
		name        string
		version     string
		base        string
		pxeBase     string
		pathVersion string
	}{
		{
			name:        "version served by the secondary",
			version:     "1.13.0",
			base:        secondaryURL,
			pxeBase:     secondaryPXEURL,
			pathVersion: "v1.13.0",
		},
		{
			name:        "strips and restores the v prefix",
			version:     "v1.13.0",
			base:        secondaryURL,
			pxeBase:     secondaryPXEURL,
			pathVersion: "v1.13.0",
		},
		{
			name:        "version served by the primary",
			version:     "1.14.0",
			base:        primaryURL,
			pxeBase:     primaryPXEURL,
			pathVersion: "v1.14.0",
		},
		{
			name:        "version without a recorded factory falls back to the primary",
			version:     "1.12.0",
			base:        primaryURL,
			pxeBase:     primaryPXEURL,
			pathVersion: "v1.12.0",
		},
		{
			name:        "unknown version falls back to the primary",
			version:     "1.11.0",
			base:        primaryURL,
			pxeBase:     primaryPXEURL,
			pathVersion: "v1.11.0",
		},
		{
			name:        "empty version uses the default",
			version:     "",
			base:        primaryURL,
			pxeBase:     primaryPXEURL,
			pathVersion: "v" + constants.DefaultTalosVersion,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			disk, err := imagefactory.ResolveBootAsset(ctx, st, tt.version, diskSpec(), schematicID, false)
			require.NoError(t, err)
			require.Equal(t, tt.base+"/image/"+schematicID+"/"+tt.pathVersion+"/nocloud-amd64.raw.xz", disk.URL)
			require.Empty(t, disk.Headers)

			pxe, err := imagefactory.ResolveBootAsset(ctx, st, tt.version, pxeSpec(), schematicID, false)
			require.NoError(t, err)
			require.Equal(t, tt.pxeBase+"/pxe/"+schematicID+"/"+tt.pathVersion+"/metal-amd64", pxe.URL)
		})
	}
}

// TestResolveBootAssetDefaults covers an Omni that reports no factory URLs at all: the public factory
// fills in, and the PXE endpoint is derived from it the way Omni itself derives an unconfigured one.
func TestResolveBootAssetDefaults(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	st := newTestState(t)

	createFeaturesConfig(ctx, t, st, func(config *omni.FeaturesConfig) {
		config.TypedSpec().Value.ImageFactoryBaseUrl = ""
		config.TypedSpec().Value.ImageFactoryPxeBaseUrl = ""
	})

	disk, err := imagefactory.ResolveBootAsset(ctx, st, "1.13.0", diskSpec(), schematicID, false)
	require.NoError(t, err)
	require.Equal(t, constants.ImageFactoryBaseURL+"/image/"+schematicID+"/v1.13.0/nocloud-amd64.raw.xz", disk.URL)
	require.Equal(t, "factory.talos.dev", disk.ImageFactoryHost)

	pxe, err := imagefactory.ResolveBootAsset(ctx, st, "1.13.0", pxeSpec(), schematicID, false)
	require.NoError(t, err)
	require.Equal(t, "https://pxe.factory.talos.dev/pxe/"+schematicID+"/v1.13.0/metal-amd64", pxe.URL)
	require.Equal(t, "pxe.factory.talos.dev", pxe.ImageFactoryHost)
}

// TestResolveBootAssetStorageKey pins the property a provider storing what it downloads depends on: the
// key follows the asset, not the credentials in its URL. Hashing the URL instead would rename everything
// stored on every password rotation and orphan the old copies.
func TestResolveBootAssetStorageKey(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	keyFor := func(t *testing.T, configure func(*omni.FeaturesConfig), version string, spec imagefactory.AssetSpec, schematic string) string {
		t.Helper()

		st := newTestState(t)

		createFeaturesConfig(ctx, t, st, configure)

		asset, err := imagefactory.ResolveBootAsset(ctx, st, version, spec, schematic, false)
		require.NoError(t, err)
		require.NotEmpty(t, asset.StorageKey)

		return asset.StorageKey
	}

	base := keyFor(t, nil, "1.13.0", diskSpec(), schematicID)

	t.Run("stable across credential rotation", func(t *testing.T) {
		t.Parallel()

		// The same asset behind an authenticated factory, and behind one whose password then changes.
		first := func(password string) string {
			st := newTestState(t)

			createFeaturesConfig(ctx, t, st, nil)
			createFactoryAuth(ctx, t, st, primaryURL, "user", password)

			asset, err := imagefactory.ResolveBootAsset(ctx, st, "1.13.0", diskSpec(), schematicID, true)
			require.NoError(t, err)
			require.Contains(t, asset.URL, password, "the standalone URL is expected to carry the password")

			return asset.StorageKey
		}

		require.Equal(t, first("secret1"), first("secret2"))
		require.Equal(t, base, first("secret1"), "credentials must not enter the key at all")
	})

	t.Run("stable across the auth placement", func(t *testing.T) {
		t.Parallel()

		st := newTestState(t)

		createFeaturesConfig(ctx, t, st, nil)
		createFactoryAuth(ctx, t, st, primaryURL, "user", "hunter2")

		headers, err := imagefactory.ResolveBootAsset(ctx, st, "1.13.0", diskSpec(), schematicID, false)
		require.NoError(t, err)

		standalone, err := imagefactory.ResolveBootAsset(ctx, st, "1.13.0", diskSpec(), schematicID, true)
		require.NoError(t, err)

		require.Equal(t, headers.StorageKey, standalone.StorageKey)
	})

	t.Run("changes with the asset", func(t *testing.T) {
		t.Parallel()

		otherSchematic := "0000000000000000000000000000000000000000000000000000000000000000"

		otherFormat := diskSpec()
		otherFormat.Format = "qcow2"

		otherArch := diskSpec()
		otherArch.Architecture = "arm64"

		secureBoot := diskSpec()
		secureBoot.SecureBoot = true

		for name, key := range map[string]string{
			"schematic":     keyFor(t, nil, "1.13.0", diskSpec(), otherSchematic),
			"talos version": keyFor(t, nil, "1.14.0", diskSpec(), schematicID),
			"format":        keyFor(t, nil, "1.13.0", otherFormat, schematicID),
			"architecture":  keyFor(t, nil, "1.13.0", otherArch, schematicID),
			"secure boot":   keyFor(t, nil, "1.13.0", secureBoot, schematicID),
			"kind":          keyFor(t, nil, "1.13.0", pxeSpec(), schematicID),
			"factory": keyFor(t, func(config *omni.FeaturesConfig) {
				config.TypedSpec().Value.ImageFactoryBaseUrl = "https://other.example.org"
			}, "1.13.0", diskSpec(), schematicID),
		} {
			require.NotEqual(t, base, key, "a different %s must produce a different key", name)
		}
	})

	t.Run("ignores a trailing slash on the factory URL", func(t *testing.T) {
		t.Parallel()

		withSlash := keyFor(t, func(config *omni.FeaturesConfig) {
			config.TypedSpec().Value.ImageFactoryBaseUrl = primaryURL + "/"
		}, "1.13.0", diskSpec(), schematicID)

		require.Equal(t, base, withSlash)
	})
}

// TestBootAssetStringOmitsURL is what keeps a logged asset from leaking its credentials, since the URL
// can carry them as userinfo or, later, as a token in the query.
func TestBootAssetStringOmitsURL(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	st := newTestState(t)

	createFeaturesConfig(ctx, t, st, nil)
	createFactoryAuth(ctx, t, st, primaryURL, "user", "hunter2")

	asset, err := imagefactory.ResolveBootAsset(ctx, st, "1.13.0", diskSpec(), schematicID, true)
	require.NoError(t, err)
	require.Contains(t, asset.URL, "hunter2", "the standalone URL is expected to carry the password")

	for _, rendered := range []string{
		asset.String(),
		// The accident this guards against: an asset handed to a formatter or a logger.
		fmt.Sprintf("%v", asset),
	} {
		require.NotContains(t, rendered, "hunter2")
		require.NotContains(t, rendered, asset.URL)
		require.Contains(t, rendered, asset.SchematicID)
		require.Contains(t, rendered, asset.StorageKey)
		require.Contains(t, rendered, asset.ImageFactoryHost)
	}
}

// TestResolveBootAssetAuth is the contract providers rely on: fetch the URL sending the headers, and a
// standalone URL works on its own.
func TestResolveBootAssetAuth(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	const authorization = "Basic dXNlcjpodW50ZXIy" // user:hunter2

	t.Run("headers by default, URL left clean", func(t *testing.T) {
		t.Parallel()

		st := newTestState(t)

		createFeaturesConfig(ctx, t, st, nil)
		createFactoryAuth(ctx, t, st, primaryURL, "user", "hunter2")

		asset, err := imagefactory.ResolveBootAsset(ctx, st, "1.13.0", diskSpec(), schematicID, false)
		require.NoError(t, err)

		require.NotEmpty(t, asset.StorageKey)

		asset.StorageKey = ""
		require.Equal(t, imagefactory.BootAsset{
			URL:              primaryURL + "/image/" + schematicID + "/v1.13.0/nocloud-amd64.raw.xz",
			Headers:          http.Header{"Authorization": []string{authorization}},
			SchematicID:      schematicID,
			ImageFactoryHost: "factory.example.org",
		}, asset)
	})

	t.Run("standalone puts the credentials in the URL and returns no headers", func(t *testing.T) {
		t.Parallel()

		st := newTestState(t)

		createFeaturesConfig(ctx, t, st, nil)
		createFactoryAuth(ctx, t, st, primaryURL, "user", "hunter2")

		asset, err := imagefactory.ResolveBootAsset(ctx, st, "1.13.0", diskSpec(), schematicID, true)
		require.NoError(t, err)

		require.NotEmpty(t, asset.StorageKey)

		asset.StorageKey = ""
		require.Equal(t, imagefactory.BootAsset{
			URL:              "https://user:hunter2@factory.example.org/image/" + schematicID + "/v1.13.0/nocloud-amd64.raw.xz",
			SchematicID:      schematicID,
			ImageFactoryHost: "factory.example.org",
		}, asset)
	})

	t.Run("PXE is always standalone", func(t *testing.T) {
		t.Parallel()

		st := newTestState(t)

		createFeaturesConfig(ctx, t, st, nil)
		createFactoryAuth(ctx, t, st, primaryURL, "user", "hunter2")

		// Not requested as standalone: PXE firmware cannot send headers, so the URL has to carry them.
		asset, err := imagefactory.ResolveBootAsset(ctx, st, "1.13.0", pxeSpec(), schematicID, false)
		require.NoError(t, err)

		require.NotEmpty(t, asset.StorageKey)

		asset.StorageKey = ""
		require.Equal(t, imagefactory.BootAsset{
			URL:              "https://user:hunter2@pxe.factory.example.org/pxe/" + schematicID + "/v1.13.0/metal-amd64",
			SchematicID:      schematicID,
			ImageFactoryHost: "pxe.factory.example.org",
		}, asset)
	})

	t.Run("each factory gets its own credentials", func(t *testing.T) {
		t.Parallel()

		st := newTestState(t)

		createFeaturesConfig(ctx, t, st, func(config *omni.FeaturesConfig) {
			config.TypedSpec().Value.SecondaryImageFactoryBaseUrl = secondaryURL
			config.TypedSpec().Value.SecondaryImageFactoryPxeBaseUrl = secondaryPXEURL
		})

		createTalosVersion(ctx, t, st, "1.13.0", secondaryURL)
		createFactoryAuth(ctx, t, st, secondaryURL, "secondary-user", "secondary-pass")

		// The primary has no credentials, so a version it serves resolves anonymously.
		asset, err := imagefactory.ResolveBootAsset(ctx, st, "1.14.0", diskSpec(), schematicID, false)
		require.NoError(t, err)
		require.Empty(t, asset.Headers)

		asset, err = imagefactory.ResolveBootAsset(ctx, st, "1.13.0", diskSpec(), schematicID, false)
		require.NoError(t, err)
		require.Equal(t, secondaryURL+"/image/"+schematicID+"/v1.13.0/nocloud-amd64.raw.xz", asset.URL)
		require.NotEmpty(t, asset.Headers.Get("Authorization"))
	})

	t.Run("special characters survive both forms", func(t *testing.T) {
		t.Parallel()

		const password = "p@ss:w/rd%"

		st := newTestState(t)

		createFeaturesConfig(ctx, t, st, nil)
		createFactoryAuth(ctx, t, st, primaryURL, "user", password)

		withHeaders, err := imagefactory.ResolveBootAsset(ctx, st, "1.13.0", diskSpec(), schematicID, false)
		require.NoError(t, err)

		encoded, ok := strings.CutPrefix(withHeaders.Headers.Get("Authorization"), "Basic ")
		require.True(t, ok)

		decoded, err := base64.StdEncoding.DecodeString(encoded)
		require.NoError(t, err)
		require.Equal(t, "user:"+password, string(decoded))

		standalone, err := imagefactory.ResolveBootAsset(ctx, st, "1.13.0", diskSpec(), schematicID, true)
		require.NoError(t, err)

		parsed, err := url.Parse(standalone.URL)
		require.NoError(t, err)

		parsedPassword, ok := parsed.User.Password()
		require.True(t, ok)
		require.Equal(t, password, parsedPassword)
		require.Equal(t, "user", parsed.User.Username())
	})

	t.Run("half a credential does not look like one", func(t *testing.T) {
		t.Parallel()

		st := newTestState(t)

		createFeaturesConfig(ctx, t, st, nil)
		createFactoryAuth(ctx, t, st, primaryURL, "user", "")

		asset, err := imagefactory.ResolveBootAsset(ctx, st, "1.13.0", diskSpec(), schematicID, true)
		require.NoError(t, err)
		require.Empty(t, asset.Headers)
		require.NotContains(t, asset.URL, "user")
	})
}

// TestResolveBootAssetDeniedCredentials covers an Omni that refuses the ImageFactoryAuth read: either it
// predates the resource, or it predates infra providers being allowed to read it. The asset comes back
// anonymous, since a factory serving assets anonymously has nothing to lose by it.
func TestResolveBootAssetDeniedCredentials(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	st := newTestState(t)

	createFeaturesConfig(ctx, t, st, nil)

	asset, err := imagefactory.ResolveBootAsset(ctx, deniedAuthState{State: st}, "1.13.0", diskSpec(), schematicID, false)
	require.NoError(t, err)
	require.Equal(t, primaryURL+"/image/"+schematicID+"/v1.13.0/nocloud-amd64.raw.xz", asset.URL)
	require.Empty(t, asset.Headers)
}

func TestResolveBootAssetNoFeaturesConfig(t *testing.T) {
	t.Parallel()

	_, err := imagefactory.ResolveBootAsset(t.Context(), newTestState(t), "1.13.0", diskSpec(), schematicID, false)
	require.ErrorContains(t, err, "failed to get features config")
}

func TestResolveBootAssetRejectsInvalidSpec(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	st := newTestState(t)

	createFeaturesConfig(ctx, t, st, nil)

	spec := diskSpec()
	spec.Platform = "../../secret"

	_, err := imagefactory.ResolveBootAsset(ctx, st, "1.13.0", spec, schematicID, false)
	require.ErrorContains(t, err, "invalid platform")
}

// TestResolveBootAssetRejectsInvalidTalosVersion covers a version that is a safe path segment but not a
// version. Without the check it resolves against the wrong factory and yields a URL that can only 404,
// so the error would surface at download or PXE boot time rather than here.
func TestResolveBootAssetRejectsInvalidTalosVersion(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	for _, version := range []string{"not-a-version", "latest", "v", "1.13.x"} {
		t.Run(version, func(t *testing.T) {
			t.Parallel()

			st := newTestState(t)

			createFeaturesConfig(ctx, t, st, nil)

			_, err := imagefactory.ResolveBootAsset(ctx, st, version, diskSpec(), schematicID, false)
			require.ErrorIs(t, err, imagefactory.ErrInvalidInput)
			require.ErrorContains(t, err, "invalid Talos version")
		})
	}

	// The forms the factory does accept keep working, with or without the v prefix.
	for _, version := range []string{"1.13.0", "v1.13.0", "1.14.0-beta.0"} {
		t.Run(version, func(t *testing.T) {
			t.Parallel()

			st := newTestState(t)

			createFeaturesConfig(ctx, t, st, nil)

			asset, err := imagefactory.ResolveBootAsset(ctx, st, version, diskSpec(), schematicID, false)
			require.NoError(t, err)
			require.Contains(t, asset.URL, "/v"+strings.TrimLeft(version, "v")+"/")
		})
	}
}

// TestResolveBootAssetRejectsRelativeConfig fails a build rather than handing back a URL nothing can
// fetch. url.Parse accepts these and JoinPath extends them, so without the check the missing scheme
// would only surface at download time as "unsupported protocol scheme". A malformed PXE URL fails the
// PXE kind alone: callers that only download images still get theirs.
func TestResolveBootAssetRejectsRelativeConfig(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	t.Run("base URL", func(t *testing.T) {
		t.Parallel()

		st := newTestState(t)

		createFeaturesConfig(ctx, t, st, func(config *omni.FeaturesConfig) {
			config.TypedSpec().Value.ImageFactoryBaseUrl = "factory.example.org"
			config.TypedSpec().Value.ImageFactoryPxeBaseUrl = ""
		})

		_, err := imagefactory.ResolveBootAsset(ctx, st, "1.13.0", diskSpec(), schematicID, false)
		require.ErrorContains(t, err, `image factory URL "factory.example.org" is not absolute`)

		_, err = imagefactory.ResolveBootAsset(ctx, st, "1.13.0", pxeSpec(), schematicID, false)
		require.ErrorContains(t, err, `image factory URL "factory.example.org" is not absolute`)
	})

	t.Run("PXE URL", func(t *testing.T) {
		t.Parallel()

		st := newTestState(t)

		createFeaturesConfig(ctx, t, st, func(config *omni.FeaturesConfig) {
			config.TypedSpec().Value.ImageFactoryPxeBaseUrl = "pxe.factory.example.org"
		})

		disk, err := imagefactory.ResolveBootAsset(ctx, st, "1.13.0", diskSpec(), schematicID, false)
		require.NoError(t, err)
		require.Equal(t, primaryURL+"/image/"+schematicID+"/v1.13.0/nocloud-amd64.raw.xz", disk.URL)

		_, err = imagefactory.ResolveBootAsset(ctx, st, "1.13.0", pxeSpec(), schematicID, false)
		require.ErrorContains(t, err, `image factory URL "pxe.factory.example.org" is not absolute`)
	})
}

// TestResolveBootAssetPathShapes pins JoinPath behavior on the base URL shapes Omni can be configured
// with.
func TestResolveBootAssetPathShapes(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	for _, tt := range []struct {
		name     string
		baseURL  string
		expected string
	}{
		{
			name:     "trailing slash",
			baseURL:  primaryURL + "/",
			expected: primaryURL + "/image/" + schematicID + "/v1.13.0/nocloud-amd64.raw.xz",
		},
		{
			name:     "path prefix is preserved",
			baseURL:  primaryURL + "/factory",
			expected: primaryURL + "/factory/image/" + schematicID + "/v1.13.0/nocloud-amd64.raw.xz",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			st := newTestState(t)

			createFeaturesConfig(ctx, t, st, func(config *omni.FeaturesConfig) {
				config.TypedSpec().Value.ImageFactoryBaseUrl = tt.baseURL
			})

			asset, err := imagefactory.ResolveBootAsset(ctx, st, "1.13.0", diskSpec(), schematicID, false)
			require.NoError(t, err)
			require.Equal(t, tt.expected, asset.URL)
		})
	}
}

// TestResolveBootAssetRejectsInvalidPathSegments covers the inputs that are not part of the spec but
// still become URL path segments. JoinPath resolves "..", so an unchecked one would silently point the
// URL at another path on the factory, taking any credentials with it, and it drops the error from a
// malformed percent escape, leaving nothing but the base URL.
func TestResolveBootAssetRejectsInvalidPathSegments(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	st := newTestState(t)

	createFeaturesConfig(ctx, t, st, nil)
	createFactoryAuth(ctx, t, st, primaryURL, "user", "hunter2")

	for name, tt := range map[string]struct {
		schematicID string
		version     string
		errContains string
	}{
		"traversal in the schematic ID": {
			schematicID: "../../..",
			version:     "1.13.0",
			errContains: "invalid schematic ID",
		},
		"slash in the schematic ID": {
			schematicID: "a/../../b",
			version:     "1.13.0",
			errContains: "invalid schematic ID",
		},
		"empty schematic ID": {
			schematicID: "",
			version:     "1.13.0",
			errContains: "invalid schematic ID",
		},
		"malformed escape in the schematic ID": {
			schematicID: "%zz",
			version:     "1.13.0",
			errContains: "invalid schematic ID",
		},
		"traversal in the Talos version": {
			schematicID: schematicID,
			version:     "1.13.0/../..",
			errContains: "invalid Talos version",
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := imagefactory.ResolveBootAsset(ctx, st, tt.version, diskSpec(), tt.schematicID, true)
			require.ErrorContains(t, err, tt.errContains)
		})
	}

	// A pre-release version is a legitimate segment and has to keep working.
	asset, err := imagefactory.ResolveBootAsset(ctx, st, "1.14.0-alpha.0", diskSpec(), schematicID, false)
	require.NoError(t, err)
	require.Contains(t, asset.URL, "/v1.14.0-alpha.0/")
}
