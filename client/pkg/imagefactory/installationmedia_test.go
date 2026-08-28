// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package imagefactory_test

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/image-factory/pkg/client"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
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

// diskSpec is the medium most providers ask for.
func diskSpec() imagefactory.MediaSpec {
	return imagefactory.MediaSpec{
		Kind:         imagefactory.InstallationMediaKindDisk,
		Platform:     "nocloud",
		Architecture: "amd64",
		Format:       "raw.xz",
	}
}

func pxeSpec() imagefactory.MediaSpec {
	return imagefactory.MediaSpec{
		Kind:         imagefactory.InstallationMediaKindPXE,
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

// TestMediaSpecFilename pins the filename of every kind the image factory serves, in the forms
// its own path parser accepts.
func TestMediaSpecFilename(t *testing.T) {
	t.Parallel()

	for name, tt := range map[string]struct {
		expected string
		spec     imagefactory.MediaSpec
	}{
		"pxe": {
			spec:     imagefactory.MediaSpec{Kind: imagefactory.InstallationMediaKindPXE, Platform: "metal", Architecture: "amd64"},
			expected: "metal-amd64",
		},
		"pxe secure boot": {
			spec:     imagefactory.MediaSpec{Kind: imagefactory.InstallationMediaKindPXE, Platform: "metal", Architecture: "amd64", SecureBoot: true},
			expected: "metal-amd64-secureboot",
		},
		"iso": {
			spec:     imagefactory.MediaSpec{Kind: imagefactory.InstallationMediaKindISO, Platform: "metal", Architecture: "arm64"},
			expected: "metal-arm64.iso",
		},
		"iso secure boot": {
			spec:     imagefactory.MediaSpec{Kind: imagefactory.InstallationMediaKindISO, Platform: "metal", Architecture: "amd64", SecureBoot: true},
			expected: "metal-amd64-secureboot.iso",
		},
		"disk raw xz": {
			spec:     imagefactory.MediaSpec{Kind: imagefactory.InstallationMediaKindDisk, Platform: "nocloud", Architecture: "amd64", Format: "raw.xz"},
			expected: "nocloud-amd64.raw.xz",
		},
		"disk qcow2": {
			spec:     imagefactory.MediaSpec{Kind: imagefactory.InstallationMediaKindDisk, Platform: "nocloud", Architecture: "amd64", Format: "qcow2"},
			expected: "nocloud-amd64.qcow2",
		},
		"disk compressed with secure boot": {
			spec:     imagefactory.MediaSpec{Kind: imagefactory.InstallationMediaKindDisk, Platform: "nocloud", Architecture: "amd64", Format: "qcow2.gz", SecureBoot: true},
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

// TestMediaSpecValidate covers what must not reach the factory as a URL path, and the per-kind
// requirements. The traversal cases are the ones that matter: these fields become path segments and
// generally come from a provider's own configuration.
func TestMediaSpecValidate(t *testing.T) {
	t.Parallel()

	for name, tt := range map[string]struct {
		errContains string
		spec        imagefactory.MediaSpec
	}{
		"kind unset": {
			spec:        imagefactory.MediaSpec{Platform: "metal", Architecture: "amd64"},
			errContains: "installation media kind is not set",
		},
		"unknown kind": {
			spec:        imagefactory.MediaSpec{Kind: "raw", Platform: "metal", Architecture: "amd64"},
			errContains: `unknown installation media kind "raw"`,
		},
		"missing architecture": {
			spec:        imagefactory.MediaSpec{Kind: imagefactory.InstallationMediaKindISO, Platform: "metal"},
			errContains: "invalid architecture",
		},
		"missing platform": {
			spec:        imagefactory.MediaSpec{Kind: imagefactory.InstallationMediaKindISO, Architecture: "amd64"},
			errContains: "invalid platform",
		},
		"platform traversal": {
			spec:        imagefactory.MediaSpec{Kind: imagefactory.InstallationMediaKindISO, Platform: "../../secret", Architecture: "amd64"},
			errContains: "invalid platform",
		},
		"architecture with a slash": {
			spec:        imagefactory.MediaSpec{Kind: imagefactory.InstallationMediaKindISO, Platform: "metal", Architecture: "amd64/../.."},
			errContains: "invalid architecture",
		},
		"format traversal": {
			spec:        imagefactory.MediaSpec{Kind: imagefactory.InstallationMediaKindDisk, Platform: "nocloud", Architecture: "amd64", Format: "../../etc/passwd"},
			errContains: "invalid disk image format",
		},
		"disk without a format": {
			spec:        imagefactory.MediaSpec{Kind: imagefactory.InstallationMediaKindDisk, Platform: "nocloud", Architecture: "amd64"},
			errContains: "invalid disk image format",
		},
		"parent directory as a platform": {
			spec:        imagefactory.MediaSpec{Kind: imagefactory.InstallationMediaKindISO, Platform: "..", Architecture: "amd64"},
			errContains: "invalid platform",
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			require.ErrorContains(t, tt.spec.Validate(), tt.errContains)
		})
	}
}

// TestResolveInstallationMediaRouting pins which factory serves each Talos version, and that the version
// segment lands in the URL in the factory's canonical v-prefixed form regardless of how the caller
// spells it. The PXE kind goes to the PXE endpoint, everything else to the image endpoint.
func TestResolveInstallationMediaRouting(t *testing.T) {
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

			disk, err := imagefactory.ResolveInstallationMedia(ctx, st, tt.version, diskSpec(), schematicID, false)
			require.NoError(t, err)
			require.Equal(t, tt.base+"/image/"+schematicID+"/"+tt.pathVersion+"/nocloud-amd64.raw.xz", disk.URL)
			require.Empty(t, disk.Headers)

			pxe, err := imagefactory.ResolveInstallationMedia(ctx, st, tt.version, pxeSpec(), schematicID, false)
			require.NoError(t, err)
			require.Equal(t, tt.pxeBase+"/pxe/"+schematicID+"/"+tt.pathVersion+"/metal-amd64", pxe.URL)
		})
	}
}

// TestResolveInstallationMediaDefaults covers an Omni that reports no factory URLs at all: the public factory
// fills in, and the PXE endpoint is derived from it the way Omni itself derives an unconfigured one.
func TestResolveInstallationMediaDefaults(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	st := newTestState(t)

	createFeaturesConfig(ctx, t, st, func(config *omni.FeaturesConfig) {
		config.TypedSpec().Value.ImageFactoryBaseUrl = ""
		config.TypedSpec().Value.ImageFactoryPxeBaseUrl = ""
	})

	disk, err := imagefactory.ResolveInstallationMedia(ctx, st, "1.13.0", diskSpec(), schematicID, false)
	require.NoError(t, err)
	require.Equal(t, constants.ImageFactoryBaseURL+"/image/"+schematicID+"/v1.13.0/nocloud-amd64.raw.xz", disk.URL)
	require.Equal(t, "factory.talos.dev", disk.ImageFactoryHost)

	pxe, err := imagefactory.ResolveInstallationMedia(ctx, st, "1.13.0", pxeSpec(), schematicID, false)
	require.NoError(t, err)
	require.Equal(t, "https://pxe.factory.talos.dev/pxe/"+schematicID+"/v1.13.0/metal-amd64", pxe.URL)
	require.Equal(t, "pxe.factory.talos.dev", pxe.ImageFactoryHost)
}

// TestResolveInstallationMediaStorageKey pins the property a provider storing what it downloads depends on: the
// key follows the medium, not the credentials in its URL. Hashing the URL instead would rename everything
// stored on every password rotation and orphan the old copies.
func TestResolveInstallationMediaStorageKey(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	keyFor := func(t *testing.T, configure func(*omni.FeaturesConfig), version string, spec imagefactory.MediaSpec, schematic string) string {
		t.Helper()

		st := newTestState(t)

		createFeaturesConfig(ctx, t, st, configure)

		media, err := imagefactory.ResolveInstallationMedia(ctx, st, version, spec, schematic, false)
		require.NoError(t, err)
		require.NotEmpty(t, media.StorageKey)

		return media.StorageKey
	}

	base := keyFor(t, nil, "1.13.0", diskSpec(), schematicID)

	t.Run("stable across credential rotation", func(t *testing.T) {
		t.Parallel()

		// The same medium behind an authenticated factory, and behind one whose password then changes.
		first := func(password string) string {
			st := newTestState(t)

			createFeaturesConfig(ctx, t, st, nil)
			createFactoryAuth(ctx, t, st, primaryURL, "user", password)

			media, err := imagefactory.ResolveInstallationMedia(ctx, st, "1.13.0", diskSpec(), schematicID, true)
			require.NoError(t, err)
			require.Contains(t, media.URL, password, "the standalone URL is expected to carry the password")

			return media.StorageKey
		}

		require.Equal(t, first("secret1"), first("secret2"))
		require.Equal(t, base, first("secret1"), "credentials must not enter the key at all")
	})

	t.Run("stable across the auth placement", func(t *testing.T) {
		t.Parallel()

		st := newTestState(t)

		createFeaturesConfig(ctx, t, st, nil)
		createFactoryAuth(ctx, t, st, primaryURL, "user", "hunter2")

		headers, err := imagefactory.ResolveInstallationMedia(ctx, st, "1.13.0", diskSpec(), schematicID, false)
		require.NoError(t, err)

		standalone, err := imagefactory.ResolveInstallationMedia(ctx, st, "1.13.0", diskSpec(), schematicID, true)
		require.NoError(t, err)

		require.Equal(t, headers.StorageKey, standalone.StorageKey)
	})

	t.Run("changes with the medium", func(t *testing.T) {
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

// TestInstallationMediaStringOmitsURL is what keeps a logged medium from leaking its credentials, since the URL
// can carry them as userinfo or, later, as a token in the query.
func TestInstallationMediaStringOmitsURL(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	st := newTestState(t)

	createFeaturesConfig(ctx, t, st, nil)
	createFactoryAuth(ctx, t, st, primaryURL, "user", "hunter2")

	media, err := imagefactory.ResolveInstallationMedia(ctx, st, "1.13.0", diskSpec(), schematicID, true)
	require.NoError(t, err)
	require.Contains(t, media.URL, "hunter2", "the standalone URL is expected to carry the password")

	for _, rendered := range []string{
		media.String(),
		// The accident this guards against: a medium handed to a formatter or a logger.
		fmt.Sprintf("%v", media),
	} {
		require.NotContains(t, rendered, "hunter2")
		require.NotContains(t, rendered, media.URL)
		require.Contains(t, rendered, media.SchematicID)
		require.Contains(t, rendered, media.StorageKey)
		require.Contains(t, rendered, media.ImageFactoryHost)
	}
}

// TestResolveInstallationMediaAuth is the contract providers rely on: fetch the URL sending the headers, and a
// standalone URL works on its own.
func TestResolveInstallationMediaAuth(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	const authorization = "Basic dXNlcjpodW50ZXIy" // user:hunter2

	t.Run("headers by default, URL left clean", func(t *testing.T) {
		t.Parallel()

		st := newTestState(t)

		createFeaturesConfig(ctx, t, st, nil)
		createFactoryAuth(ctx, t, st, primaryURL, "user", "hunter2")

		media, err := imagefactory.ResolveInstallationMedia(ctx, st, "1.13.0", diskSpec(), schematicID, false)
		require.NoError(t, err)

		require.NotEmpty(t, media.StorageKey)

		media.StorageKey = ""
		require.Equal(t, imagefactory.InstallationMedia{
			URL:              primaryURL + "/image/" + schematicID + "/v1.13.0/nocloud-amd64.raw.xz",
			Headers:          http.Header{"Authorization": []string{authorization}},
			SchematicID:      schematicID,
			ImageFactoryHost: "factory.example.org",
		}, media)
	})

	t.Run("standalone puts the credentials in the URL and returns no headers", func(t *testing.T) {
		t.Parallel()

		st := newTestState(t)

		createFeaturesConfig(ctx, t, st, nil)
		createFactoryAuth(ctx, t, st, primaryURL, "user", "hunter2")

		media, err := imagefactory.ResolveInstallationMedia(ctx, st, "1.13.0", diskSpec(), schematicID, true)
		require.NoError(t, err)

		require.NotEmpty(t, media.StorageKey)

		media.StorageKey = ""
		require.Equal(t, imagefactory.InstallationMedia{
			URL:              "https://user:hunter2@factory.example.org/image/" + schematicID + "/v1.13.0/nocloud-amd64.raw.xz",
			SchematicID:      schematicID,
			ImageFactoryHost: "factory.example.org",
		}, media)
	})

	t.Run("PXE is always standalone", func(t *testing.T) {
		t.Parallel()

		st := newTestState(t)

		createFeaturesConfig(ctx, t, st, nil)
		createFactoryAuth(ctx, t, st, primaryURL, "user", "hunter2")

		// Not requested as standalone: PXE firmware cannot send headers, so the URL has to carry them.
		media, err := imagefactory.ResolveInstallationMedia(ctx, st, "1.13.0", pxeSpec(), schematicID, false)
		require.NoError(t, err)

		require.NotEmpty(t, media.StorageKey)

		media.StorageKey = ""
		require.Equal(t, imagefactory.InstallationMedia{
			URL:              "https://user:hunter2@pxe.factory.example.org/pxe/" + schematicID + "/v1.13.0/metal-amd64",
			SchematicID:      schematicID,
			ImageFactoryHost: "pxe.factory.example.org",
		}, media)
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
		media, err := imagefactory.ResolveInstallationMedia(ctx, st, "1.14.0", diskSpec(), schematicID, false)
		require.NoError(t, err)
		require.Empty(t, media.Headers)

		media, err = imagefactory.ResolveInstallationMedia(ctx, st, "1.13.0", diskSpec(), schematicID, false)
		require.NoError(t, err)
		require.Equal(t, secondaryURL+"/image/"+schematicID+"/v1.13.0/nocloud-amd64.raw.xz", media.URL)
		require.NotEmpty(t, media.Headers.Get("Authorization"))
	})

	t.Run("special characters survive both forms", func(t *testing.T) {
		t.Parallel()

		const password = "p@ss:w/rd%"

		st := newTestState(t)

		createFeaturesConfig(ctx, t, st, nil)
		createFactoryAuth(ctx, t, st, primaryURL, "user", password)

		withHeaders, err := imagefactory.ResolveInstallationMedia(ctx, st, "1.13.0", diskSpec(), schematicID, false)
		require.NoError(t, err)

		encoded, ok := strings.CutPrefix(withHeaders.Headers.Get("Authorization"), "Basic ")
		require.True(t, ok)

		decoded, err := base64.StdEncoding.DecodeString(encoded)
		require.NoError(t, err)
		require.Equal(t, "user:"+password, string(decoded))

		standalone, err := imagefactory.ResolveInstallationMedia(ctx, st, "1.13.0", diskSpec(), schematicID, true)
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

		media, err := imagefactory.ResolveInstallationMedia(ctx, st, "1.13.0", diskSpec(), schematicID, true)
		require.NoError(t, err)
		require.Empty(t, media.Headers)
		require.NotContains(t, media.URL, "user")
	})
}

// TestResolveInstallationMediaDeniedCredentials covers an Omni that refuses the ImageFactoryAuth read: either it
// predates the resource, or it predates infra providers being allowed to read it. The media comes back
// anonymous, since a factory serving media anonymously has nothing to lose by it.
func TestResolveInstallationMediaDeniedCredentials(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	st := newTestState(t)

	createFeaturesConfig(ctx, t, st, nil)

	media, err := imagefactory.ResolveInstallationMedia(ctx, deniedAuthState{State: st}, "1.13.0", diskSpec(), schematicID, false)
	require.NoError(t, err)
	require.Equal(t, primaryURL+"/image/"+schematicID+"/v1.13.0/nocloud-amd64.raw.xz", media.URL)
	require.Empty(t, media.Headers)
}

func TestResolveInstallationMediaNoFeaturesConfig(t *testing.T) {
	t.Parallel()

	_, err := imagefactory.ResolveInstallationMedia(t.Context(), newTestState(t), "1.13.0", diskSpec(), schematicID, false)
	require.ErrorContains(t, err, "failed to get features config")
}

func TestResolveInstallationMediaRejectsInvalidSpec(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	st := newTestState(t)

	createFeaturesConfig(ctx, t, st, nil)

	spec := diskSpec()
	spec.Platform = "../../secret"

	_, err := imagefactory.ResolveInstallationMedia(ctx, st, "1.13.0", spec, schematicID, false)
	require.ErrorContains(t, err, "invalid platform")
}

// TestResolveInstallationMediaRejectsInvalidTalosVersion covers a version that is a safe path segment but not a
// version. Without the check it resolves against the wrong factory and yields a URL that can only 404,
// so the error would surface at download or PXE boot time rather than here.
func TestResolveInstallationMediaRejectsInvalidTalosVersion(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	for _, version := range []string{"not-a-version", "latest", "v", "1.13.x"} {
		t.Run(version, func(t *testing.T) {
			t.Parallel()

			st := newTestState(t)

			createFeaturesConfig(ctx, t, st, nil)

			_, err := imagefactory.ResolveInstallationMedia(ctx, st, version, diskSpec(), schematicID, false)
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

			media, err := imagefactory.ResolveInstallationMedia(ctx, st, version, diskSpec(), schematicID, false)
			require.NoError(t, err)
			require.Contains(t, media.URL, "/v"+strings.TrimLeft(version, "v")+"/")
		})
	}
}

// TestResolveInstallationMediaRejectsRelativeConfig fails a build rather than handing back a URL nothing can
// fetch. url.Parse accepts these and JoinPath extends them, so without the check the missing scheme
// would only surface at download time as "unsupported protocol scheme". A malformed PXE URL fails the
// PXE kind alone: callers that only download images still get theirs.
func TestResolveInstallationMediaRejectsRelativeConfig(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	t.Run("base URL", func(t *testing.T) {
		t.Parallel()

		st := newTestState(t)

		createFeaturesConfig(ctx, t, st, func(config *omni.FeaturesConfig) {
			config.TypedSpec().Value.ImageFactoryBaseUrl = "factory.example.org"
			config.TypedSpec().Value.ImageFactoryPxeBaseUrl = ""
		})

		_, err := imagefactory.ResolveInstallationMedia(ctx, st, "1.13.0", diskSpec(), schematicID, false)
		require.ErrorContains(t, err, `image factory URL "factory.example.org" is not absolute`)

		_, err = imagefactory.ResolveInstallationMedia(ctx, st, "1.13.0", pxeSpec(), schematicID, false)
		require.ErrorContains(t, err, `image factory URL "factory.example.org" is not absolute`)
	})

	t.Run("PXE URL", func(t *testing.T) {
		t.Parallel()

		st := newTestState(t)

		createFeaturesConfig(ctx, t, st, func(config *omni.FeaturesConfig) {
			config.TypedSpec().Value.ImageFactoryPxeBaseUrl = "pxe.factory.example.org"
		})

		disk, err := imagefactory.ResolveInstallationMedia(ctx, st, "1.13.0", diskSpec(), schematicID, false)
		require.NoError(t, err)
		require.Equal(t, primaryURL+"/image/"+schematicID+"/v1.13.0/nocloud-amd64.raw.xz", disk.URL)

		_, err = imagefactory.ResolveInstallationMedia(ctx, st, "1.13.0", pxeSpec(), schematicID, false)
		require.ErrorContains(t, err, `image factory URL "pxe.factory.example.org" is not absolute`)
	})
}

// TestResolveInstallationMediaPathShapes pins JoinPath behavior on the base URL shapes Omni can be configured
// with.
func TestResolveInstallationMediaPathShapes(t *testing.T) {
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

			media, err := imagefactory.ResolveInstallationMedia(ctx, st, "1.13.0", diskSpec(), schematicID, false)
			require.NoError(t, err)
			require.Equal(t, tt.expected, media.URL)
		})
	}
}

// TestResolveInstallationMediaRejectsInvalidPathSegments covers the inputs that are not part of the spec but
// still become URL path segments. JoinPath resolves "..", so an unchecked one would silently point the
// URL at another path on the factory, taking any credentials with it, and it drops the error from a
// malformed percent escape, leaving nothing but the base URL.
func TestResolveInstallationMediaRejectsInvalidPathSegments(t *testing.T) {
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

			_, err := imagefactory.ResolveInstallationMedia(ctx, st, tt.version, diskSpec(), tt.schematicID, true)
			require.ErrorContains(t, err, tt.errContains)
		})
	}

	// A pre-release version is a legitimate segment and has to keep working.
	media, err := imagefactory.ResolveInstallationMedia(ctx, st, "1.14.0-alpha.0", diskSpec(), schematicID, false)
	require.NoError(t, err)
	require.Contains(t, media.URL, "/v1.14.0-alpha.0/")
}

// fakeFactoryClient is an imagefactory.FactoryClient that does only the two things resolving an installation medium media
// asks of one: report the URL it serves, so ForURL can find it, and issue a download token.
type fakeFactoryClient struct {
	imagefactory.FactoryClient

	err   error
	token string
	url   string

	// onRequest runs while the token request is in flight, for a test that has to change something under it,
	// such as canceling the caller.
	onRequest func()

	deadline time.Time

	requests []time.Duration

	hasDeadline bool

	mu sync.Mutex
}

func (f *fakeFactoryClient) URL() string { return f.url }

func (f *fakeFactoryClient) DownloadToken(ctx context.Context, ttl time.Duration) (string, error) {
	f.mu.Lock()
	f.requests = append(f.requests, ttl)
	f.deadline, f.hasDeadline = ctx.Deadline()
	f.mu.Unlock()

	if f.onRequest != nil {
		f.onRequest()
	}

	return f.token, f.err
}

// calls returns the lifetimes DownloadToken was asked for, in order.
func (f *fakeFactoryClient) calls() []time.Duration {
	f.mu.Lock()
	defer f.mu.Unlock()

	return slices.Clone(f.requests)
}

// tokenDeadline returns the deadline the last token request ran under.
func (f *fakeFactoryClient) tokenDeadline() (time.Time, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.deadline, f.hasDeadline
}

// newIssuer is a fake configured for the primary factory, which is the one every case here resolves
// against unless it is testing what happens when none is.
func newIssuer(token string, err error) *fakeFactoryClient {
	return &fakeFactoryClient{url: primaryURL, token: token, err: err}
}

func withIssuer(t *testing.T, st state.State, issuer imagefactory.FactoryClient, ttl time.Duration) imagefactory.ResolveOption {
	t.Helper()

	return imagefactory.WithDownloadTokens(imagefactory.DownloadTokenOptions{
		Factories: imagefactory.NewClients(st, issuer),
		Logger:    zaptest.NewLogger(t),
		TTL:       ttl,
	})
}

// authenticatedState is the common setup: a primary factory Omni holds credentials for.
func authenticatedState(ctx context.Context, t *testing.T) state.State {
	t.Helper()

	st := newTestState(t)

	createFeaturesConfig(ctx, t, st, nil)
	createFactoryAuth(ctx, t, st, primaryURL, "user", "hunter2")

	return st
}

// TestResolveInstallationMediaDownloadTokenRequest covers the deadline the token request runs under, and what a
// caller gets when its own context ends first.
func TestResolveInstallationMediaDownloadTokenRequest(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	const (
		token = "eyJhbGciOiJFUzI1NiJ9.fake.token"

		// requestTimeout pins imagefactory's unexported deadline for the token request.
		requestTimeout = 30 * time.Second
	)

	t.Run("the request runs under a deadline of its own", func(t *testing.T) {
		t.Parallel()

		st := authenticatedState(ctx, t)
		issuer := newIssuer(token, nil)

		before := time.Now()

		_, err := imagefactory.ResolveInstallationMedia(ctx, st, "1.13.0", diskSpec(), schematicID, false, withIssuer(t, st, issuer, 0))
		require.NoError(t, err)

		// Without one, a factory that accepts the connection and never answers holds the resolve for the factory
		// client's own timeout, which is half an hour.
		deadline, ok := issuer.tokenDeadline()
		require.True(t, ok)
		require.WithinRange(t, deadline, before.Add(requestTimeout), time.Now().Add(requestTimeout))
	})

	t.Run("a caller that goes away is not answered with credentials", func(t *testing.T) {
		t.Parallel()

		st := authenticatedState(ctx, t)

		callerCtx, cancel := context.WithCancel(ctx)
		t.Cleanup(cancel)

		// Canceled while the request is in flight, which is what a caller hanging up looks like.
		issuer := newIssuer("", errors.New("connection reset"))
		issuer.onRequest = cancel

		_, err := imagefactory.ResolveInstallationMedia(callerCtx, st, "1.13.0", diskSpec(), schematicID, true, withIssuer(t, st, issuer, 0))
		require.ErrorIs(t, err, context.Canceled)
	})
}

// TestResolveInstallationMediaPXEDownloadToken covers the authentication a caller gets for a PXE script.
// The factory forwards the token that fetched the script into the kernel and initramfs URLs it points at,
// so the token authenticates the whole boot and the credentials never leave Omni.
func TestResolveInstallationMediaPXEDownloadToken(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	const (
		token = "eyJhbGciOiJFUzI1NiJ9.fake.token"

		// defaultPXETTL pins imagefactory's unexported PXE default, which is longer than the image one
		// because a PXE URL is written into a boot configuration before anything boots from it.
		defaultPXETTL = 2 * time.Hour
	)

	t.Run("a token replaces the userinfo", func(t *testing.T) {
		t.Parallel()

		st := authenticatedState(ctx, t)
		issuer := newIssuer(token, nil)

		before := time.Now()

		media, err := imagefactory.ResolveInstallationMedia(ctx, st, "1.13.0", pxeSpec(), schematicID, false, withIssuer(t, st, issuer, 0))
		require.NoError(t, err)

		require.Equal(t, primaryPXEURL+"/pxe/"+schematicID+"/v1.13.0/metal-amd64?token="+url.QueryEscape(token), media.URL)
		require.Empty(t, media.Headers, "PXE firmware cannot send a header")
		require.NotContains(t, media.URL, "hunter2", "the credential must not travel alongside the token")

		require.Equal(t, []time.Duration{defaultPXETTL}, issuer.calls())
		require.WithinRange(t, media.ExpiresAt, before.Add(defaultPXETTL), time.Now().Add(defaultPXETTL))
	})

	t.Run("a requested lifetime overrides the PXE default", func(t *testing.T) {
		t.Parallel()

		st := authenticatedState(ctx, t)
		issuer := newIssuer(token, nil)

		_, err := imagefactory.ResolveInstallationMedia(ctx, st, "1.13.0", pxeSpec(), schematicID, false, withIssuer(t, st, issuer, 30*time.Minute))
		require.NoError(t, err)

		require.Equal(t, []time.Duration{30 * time.Minute}, issuer.calls())
	})

	t.Run("a factory that issues no token falls back to credentials", func(t *testing.T) {
		t.Parallel()

		st := authenticatedState(ctx, t)
		issuer := newIssuer("", &client.HTTPError{Code: http.StatusNotFound, Message: "not found"})

		// An image factory below 1.6.0 rejects a token on /pxe/, and one below 1.5.0 or with its own
		// authentication disabled issues none at all. Both answer 404 here.
		media, err := imagefactory.ResolveInstallationMedia(ctx, st, "1.13.0", pxeSpec(), schematicID, false, withIssuer(t, st, issuer, 0))
		require.NoError(t, err)

		require.Equal(t, "https://user:hunter2@pxe.factory.example.org/pxe/"+schematicID+"/v1.13.0/metal-amd64", media.URL)
		require.Empty(t, media.Headers)
		require.Zero(t, media.ExpiresAt)
	})
}

// TestResolveInstallationMediaDownloadToken covers the authentication a caller gets for a medium under /image/
// when the factory issues download tokens, and the credential fallback for every factory that does not.
func TestResolveInstallationMediaDownloadToken(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	const (
		token         = "eyJhbGciOiJFUzI1NiJ9.fake.token"
		authorization = "Basic dXNlcjpodW50ZXIy" // user:hunter2

		// defaultTTL pins imagefactory's unexported default for an image, which matches the factory's own documented
		// default. Omni sends it explicitly so that the expiry it reports back is knowable.
		defaultTTL = 5 * time.Minute
	)

	mediaPath := "/image/" + schematicID + "/v1.13.0/nocloud-amd64.raw.xz"

	t.Run("a token replaces the userinfo on a standalone image", func(t *testing.T) {
		t.Parallel()

		st := authenticatedState(ctx, t)
		issuer := newIssuer(token, nil)

		media, err := imagefactory.ResolveInstallationMedia(ctx, st, "1.13.0", diskSpec(), schematicID, true, withIssuer(t, st, issuer, 0))
		require.NoError(t, err)

		require.Equal(t, primaryURL+mediaPath+"?token="+url.QueryEscape(token), media.URL)
		require.Empty(t, media.Headers)
		require.NotContains(t, media.URL, "hunter2", "the credential must not travel alongside the token")
	})

	t.Run("a token replaces the header on a non-standalone image", func(t *testing.T) {
		t.Parallel()

		st := authenticatedState(ctx, t)
		issuer := newIssuer(token, nil)

		media, err := imagefactory.ResolveInstallationMedia(ctx, st, "1.13.0", diskSpec(), schematicID, false, withIssuer(t, st, issuer, 0))
		require.NoError(t, err)

		require.Equal(t, primaryURL+mediaPath+"?token="+url.QueryEscape(token), media.URL)
		require.Empty(t, media.Headers, "a token in the URL leaves nothing for the headers to carry")
	})

	t.Run("an anonymous factory never asks for a token", func(t *testing.T) {
		t.Parallel()

		st := newTestState(t)
		createFeaturesConfig(ctx, t, st, nil)

		issuer := newIssuer(token, nil)

		media, err := imagefactory.ResolveInstallationMedia(ctx, st, "1.13.0", diskSpec(), schematicID, false, withIssuer(t, st, issuer, 0))
		require.NoError(t, err)

		require.Equal(t, primaryURL+mediaPath, media.URL)
		require.Empty(t, media.Headers)
		require.Empty(t, issuer.calls(), "there is nothing to authenticate")
		require.Zero(t, media.ExpiresAt)
	})

	t.Run("the lifetime asked for reaches the factory and comes back as the expiry", func(t *testing.T) {
		t.Parallel()

		for name, tt := range map[string]struct {
			requested time.Duration
			expected  time.Duration
		}{
			// Never zero: a lifetime Omni did not send is a lifetime Omni cannot report back as an expiry.
			"nothing requested takes Omni's default": {requested: 0, expected: defaultTTL},
			"a requested lifetime is sent unchanged": {requested: 90 * time.Minute, expected: 90 * time.Minute},
		} {
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				st := authenticatedState(ctx, t)
				issuer := newIssuer(token, nil)

				before := time.Now()

				media, err := imagefactory.ResolveInstallationMedia(ctx, st, "1.13.0", diskSpec(), schematicID, false, withIssuer(t, st, issuer, tt.requested))
				require.NoError(t, err)

				require.Equal(t, []time.Duration{tt.expected}, issuer.calls())
				require.WithinRange(t, media.ExpiresAt, before.Add(tt.expected), time.Now().Add(tt.expected))
			})
		}
	})

	t.Run("a factory that does not issue tokens falls back to credentials", func(t *testing.T) {
		t.Parallel()

		// 404 is a factory below 1.5.0, 405 a route that exists for other methods.
		// Neither is a failure: it is what most deployments look like.
		for name, err := range map[string]error{
			"404": &client.HTTPError{Code: http.StatusNotFound, Message: "not found"},
			"405": &client.HTTPError{Code: http.StatusMethodNotAllowed, Message: "method not allowed"},
			// A transient problem must not fail the resolve, since falling back is never worse than what a
			// factory without token support gets.
			"a transient failure": errors.New("connection reset"),
		} {
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				st := authenticatedState(ctx, t)

				standalone, resolveErr := imagefactory.ResolveInstallationMedia(ctx, st, "1.13.0", diskSpec(), schematicID, true, withIssuer(t, st, newIssuer("", err), 0))
				require.NoError(t, resolveErr)
				require.Equal(t, "https://user:hunter2@factory.example.org"+mediaPath, standalone.URL)
				require.Zero(t, standalone.ExpiresAt)

				withHeaders, resolveErr := imagefactory.ResolveInstallationMedia(ctx, st, "1.13.0", diskSpec(), schematicID, false, withIssuer(t, st, newIssuer("", err), 0))
				require.NoError(t, resolveErr)
				require.Equal(t, primaryURL+mediaPath, withHeaders.URL)
				require.Equal(t, authorization, withHeaders.Headers.Get("Authorization"))
				require.Zero(t, withHeaders.ExpiresAt)
			})
		}
	})

	t.Run("a refused lifetime the caller asked for is the caller's error", func(t *testing.T) {
		t.Parallel()

		st := authenticatedState(ctx, t)

		// The factory refuses anything outside its configured bounds with a 400. Silently handing back a
		// long-lived credential instead would answer a request the caller did not make.
		_, err := imagefactory.ResolveInstallationMedia(ctx, st, "1.13.0", diskSpec(), schematicID, false,
			withIssuer(t, st, newIssuer("", &client.InvalidSchematicError{}), 100*time.Hour))

		require.ErrorIs(t, err, imagefactory.ErrInvalidInput)
		require.Contains(t, err.Error(), "100h0m0s")
	})

	t.Run("a refused default lifetime is not the caller's error", func(t *testing.T) {
		t.Parallel()

		st := authenticatedState(ctx, t)

		// Omni's own guess is not the caller's request, so a factory whose bounds exclude it falls back
		// rather than failing a provision over a value the caller never chose.
		media, err := imagefactory.ResolveInstallationMedia(ctx, st, "1.13.0", diskSpec(), schematicID, false,
			withIssuer(t, st, newIssuer("", &client.InvalidSchematicError{}), 0))

		require.NoError(t, err)
		require.Equal(t, authorization, media.Headers.Get("Authorization"))
		require.Zero(t, media.ExpiresAt)
	})

	t.Run("a 200 carrying no token falls back", func(t *testing.T) {
		t.Parallel()

		st := authenticatedState(ctx, t)

		// Placing an empty token would build a URL with no credentials anywhere, which only fails at the
		// download, with nothing in Omni's logs pointing at the token.
		media, err := imagefactory.ResolveInstallationMedia(ctx, st, "1.13.0", diskSpec(), schematicID, true, withIssuer(t, st, newIssuer("", nil), 0))
		require.NoError(t, err)

		require.Equal(t, "https://user:hunter2@factory.example.org"+mediaPath, media.URL)
		require.NotContains(t, media.URL, "token=")
		require.Zero(t, media.ExpiresAt)
	})

	t.Run("an unroutable factory falls back", func(t *testing.T) {
		t.Parallel()

		st := authenticatedState(ctx, t)

		// A token must come from the factory serving the medium and no other, so a client set with nothing
		// configured for it issues nothing at all.
		issuer := &fakeFactoryClient{url: "https://other.example.org", token: token}

		media, err := imagefactory.ResolveInstallationMedia(ctx, st, "1.13.0", diskSpec(), schematicID, false, withIssuer(t, st, issuer, 0))
		require.NoError(t, err)

		require.Equal(t, authorization, media.Headers.Get("Authorization"))
		require.Empty(t, issuer.calls())
	})

	t.Run("a resolve with no token issuer behaves as it did before tokens", func(t *testing.T) {
		t.Parallel()

		st := authenticatedState(ctx, t)

		// This is the provider-side fallback against an Omni that predates the installation media API.
		media, err := imagefactory.ResolveInstallationMedia(ctx, st, "1.13.0", diskSpec(), schematicID, false)
		require.NoError(t, err)

		require.Equal(t, authorization, media.Headers.Get("Authorization"))
		require.Zero(t, media.ExpiresAt)
	})

	t.Run("the storage key and the log line are unaffected", func(t *testing.T) {
		t.Parallel()

		st := authenticatedState(ctx, t)

		// The key must not move when a token appears, or every provider re-downloads everything it stores.
		withToken, err := imagefactory.ResolveInstallationMedia(ctx, st, "1.13.0", diskSpec(), schematicID, false, withIssuer(t, st, newIssuer(token, nil), 0))
		require.NoError(t, err)

		withCredentials, err := imagefactory.ResolveInstallationMedia(ctx, st, "1.13.0", diskSpec(), schematicID, false)
		require.NoError(t, err)

		require.Equal(t, withCredentials.StorageKey, withToken.StorageKey)

		for _, rendered := range []string{withToken.String(), fmt.Sprintf("%v", withToken)} {
			require.NotContains(t, rendered, token, "a logged medium must not leak the token")
			require.NotContains(t, rendered, withToken.URL)
			require.Contains(t, rendered, withToken.StorageKey)
			require.Contains(t, rendered, "expires", "the expiry explains a download failing later, and is not a secret")
		}
	})
}

// TestResolveInstallationMediaNegativeTokenLifetime covers a lifetime the caller got wrong. It is refused the same
// way for every kind, not only for the ones that would have requested a token, so that a bad value
// cannot pass unnoticed against the public factory and fail only against an authenticated one.
func TestResolveInstallationMediaNegativeTokenLifetime(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	const token = "eyJhbGciOiJFUzI1NiJ9.fake.token"

	// The client sends the ttl parameter only when it is positive, so a negative one would reach the
	// factory as no lifetime at all and come back as an expiry in the past. It is refused the same way
	// for every kind, including the two that never request a token: accepting it there would let a bad
	// value through against the public factory and fail only against an authenticated one.
	for name, tt := range map[string]struct {
		spec      imagefactory.MediaSpec
		anonymous bool
	}{
		"disk, which would have requested one": {spec: diskSpec()},
		"PXE, which never requests one":        {spec: pxeSpec()},
		"an anonymous factory":                 {spec: diskSpec(), anonymous: true},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			st := newTestState(t)
			createFeaturesConfig(ctx, t, st, nil)

			if !tt.anonymous {
				createFactoryAuth(ctx, t, st, primaryURL, "user", "hunter2")
			}

			issuer := newIssuer(token, nil)

			_, err := imagefactory.ResolveInstallationMedia(ctx, st, "1.13.0", tt.spec, schematicID, false, withIssuer(t, st, issuer, -time.Hour))

			require.ErrorIs(t, err, imagefactory.ErrInvalidInput)
			require.Empty(t, issuer.calls(), "a lifetime that cannot be honored must not reach the factory")
		})
	}
}
