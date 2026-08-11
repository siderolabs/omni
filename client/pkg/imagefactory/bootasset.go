// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package imagefactory

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/talos/pkg/machinery/platforms"

	"github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// BootAssetKind selects one of the asset kinds the image factory serves.
type BootAssetKind string

const (
	// BootAssetKindPXE is an iPXE script, served from the factory's PXE endpoint.
	BootAssetKindPXE BootAssetKind = "pxe"

	// BootAssetKindISO is a bootable ISO image.
	BootAssetKindISO BootAssetKind = "iso"

	// BootAssetKindDisk is a bootable disk image, in the disk format named by AssetSpec.Format.
	BootAssetKindDisk BootAssetKind = "disk"
)

// ErrInvalidInput marks the arguments a caller got wrong, as opposed to a misconfigured image factory.
// A server turning these into a response should answer InvalidArgument for them and not for the rest.
var ErrInvalidInput = errors.New("invalid boot asset request")

// AssetSpec names a boot asset, without saying how the image factory spells it.
type AssetSpec struct {
	Kind BootAssetKind

	// Platform is the Talos platform, such as "metal" or "nocloud".
	Platform string

	// Architecture is the machine architecture, such as "amd64" or "arm64".
	Architecture string

	// Format is the disk image format of the disk kind, such as "raw.xz", "qcow2" or "qcow2.gz". It is
	// unused by every other kind, whose format the kind itself implies.
	Format string

	// SecureBoot selects the secure boot variant of the asset.
	SecureBoot bool
}

// pathSegment matches what may appear as a single URL path segment.
func validPathSegment(value string) bool {
	if value == "" {
		return false
	}

	return strings.IndexFunc(value, func(r rune) bool {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return false
		case r == '.', r == '_', r == '-':
			return false
		default:
			return true
		}
	}) == -1 && value != "." && value != ".."
}

// Validate reports whether the spec names an asset the image factory can serve.
func (s AssetSpec) Validate() error {
	switch s.Kind {
	case BootAssetKindPXE, BootAssetKindISO, BootAssetKindDisk:
	case "":
		return fmt.Errorf("%w: boot asset kind is not set", ErrInvalidInput)
	default:
		return fmt.Errorf("%w: unknown boot asset kind %q", ErrInvalidInput, s.Kind)
	}

	if !validPathSegment(s.Platform) {
		return fmt.Errorf("%w: invalid platform %q", ErrInvalidInput, s.Platform)
	}

	if !validPathSegment(s.Architecture) {
		return fmt.Errorf("%w: invalid architecture %q", ErrInvalidInput, s.Architecture)
	}

	if s.Kind == BootAssetKindDisk && !validPathSegment(s.Format) {
		return fmt.Errorf("%w: invalid disk image format %q", ErrInvalidInput, s.Format)
	}

	return nil
}

// Filename returns the name the image factory serves this asset under.
func (s AssetSpec) Filename() string {
	platform := platforms.Platform{Name: s.Platform}

	switch s.Kind {
	case BootAssetKindPXE:
		if s.SecureBoot {
			return platform.SecureBootPXEScriptPath(s.Architecture)
		}

		return platform.PXEScriptPath(s.Architecture)
	case BootAssetKindISO:
		if s.SecureBoot {
			return platform.SecureBootISOPath(s.Architecture)
		}

		return platform.ISOPath(s.Architecture)
	case BootAssetKindDisk:
		if s.SecureBoot {
			return platform.SecureBootDiskImagePath(s.Architecture, s.Format)
		}

		return platform.DiskImagePath(s.Architecture, s.Format)
	}

	return ""
}

// BootAsset is a fetchable boot asset.
//
// Fetch URL, sending Headers when they are not empty. The URL is opaque and can carry secrets, whether
// as credentials or as a factory token, so it does not belong in a log line.
type BootAsset struct {
	// Headers must be sent with the request fetching URL. Empty when the factory needs no
	// authentication, or when it travels inside the URL instead.
	Headers http.Header

	// URL of the boot asset.
	URL string

	// SchematicID is the ID the image factory gave the schematic this asset is built from.
	SchematicID string

	// StorageKey identifies this asset, for callers that store what they download. It changes only when
	// the asset itself does, so it survives a credential rotation, which the URL does not: a name derived
	// from the URL would change with the credentials in it and orphan whatever was stored under the old
	// one.
	//
	// Treat it as opaque. Omni may change how it is derived, which costs a caller one re-download of the
	// assets it already holds.
	StorageKey string

	// ImageFactoryHost is the host serving the asset, the same one in the URL. Purely informative: to
	// fetch the asset, use the URL and the headers.
	ImageFactoryHost string
}

// String returns a representation of the asset that is safe to log.
//
// The URL is left out on purpose: it can carry credentials as userinfo or a token in the query, so a
// logged BootAsset would leak them. What remains identifies the asset just as well. This covers %v and
// zap.Any, which prefer a Stringer, but not reflection or json.Marshal.
func (a BootAsset) String() string {
	return fmt.Sprintf("BootAsset{host: %q, schematic: %q, key: %q}", a.ImageFactoryHost, a.SchematicID, a.StorageKey)
}

// storageKey derives the identity of an asset from everything that decides its content: the factory
// serving it, the schematic, the Talos version and the name the factory serves it under.
//
// Deliberately not the URL, which also carries the credentials and so changes when they rotate.
//
// **Important:** New inputs must be appended only when they are set, so that every asset hashed
// before the input existed keeps its key. A changed key makes the providers
// re-download everything they already store.
func storageKey(factoryBaseURL, pathPrefix, schematicID, version, filename string) string {
	digest := sha256.Sum256([]byte(strings.Join([]string{
		normalizeFactoryURL(factoryBaseURL), pathPrefix, schematicID, version, filename,
	}, "\x00")))

	return hex.EncodeToString(digest[:])
}

// ResolveBootAsset returns the boot asset the given schematic produces, served by whichever image
// factory Omni uses for the given Talos version. An empty version means the default Talos version.
//
// Set standalone when the fetch cannot carry headers, such as a hypervisor download API handed a bare
// URL: any authentication then travels inside the URL and Headers comes back empty. The PXE kind is
// always resolved that way, since the firmware fetching the script cannot send headers either.
func ResolveBootAsset(ctx context.Context, st state.State, talosVersion string, spec AssetSpec, schematicID string, standalone bool) (BootAsset, error) {
	if err := spec.Validate(); err != nil {
		return BootAsset{}, err
	}

	if talosVersion == "" {
		talosVersion = constants.DefaultTalosVersion
	}

	if _, err := semver.ParseTolerant(talosVersion); err != nil {
		return BootAsset{}, fmt.Errorf("%w: invalid Talos version %q: %w", ErrInvalidInput, talosVersion, err)
	}

	version := "v" + strings.TrimLeft(talosVersion, "v")

	if !validPathSegment(schematicID) {
		return BootAsset{}, fmt.Errorf("%w: invalid schematic ID %q", ErrInvalidInput, schematicID)
	}

	resolved, err := resolveEndpoint(ctx, st, talosVersion)
	if err != nil {
		return BootAsset{}, err
	}

	baseURL, pathPrefix := resolved.baseURL, "image"
	if spec.Kind == BootAssetKindPXE {
		baseURL, pathPrefix = resolved.pxeBaseURL, "pxe"
	}

	parsed, err := parseFactoryURL(baseURL)
	if err != nil {
		return BootAsset{}, err
	}

	filename := spec.Filename()
	assetURL := parsed.JoinPath(pathPrefix, schematicID, version, filename)

	asset := BootAsset{
		SchematicID:      schematicID,
		StorageKey:       storageKey(baseURL, pathPrefix, schematicID, version, filename),
		ImageFactoryHost: assetURL.Host,
	}

	switch {
	case resolved.username == "" || resolved.password == "":
		// The factory serves this anonymously, so there is nothing to place.
	case standalone || spec.Kind == BootAssetKindPXE:
		assetURL.User = url.UserPassword(resolved.username, resolved.password)
	default:
		asset.Headers = http.Header{
			"Authorization": []string{"Basic " + base64.StdEncoding.EncodeToString([]byte(resolved.username+":"+resolved.password))},
		}
	}

	asset.URL = assetURL.String()

	return asset, nil
}

// parseFactoryURL parses a factory URL and rejects anything that is not absolute.
//
// url.Parse takes "" and "factory.example.org" without complaint, and JoinPath extends the relative
// reference it produces just as happily, so a missing scheme would survive all the way to whatever
// attempts the download and arrive there as "unsupported protocol scheme".
func parseFactoryURL(rawURL string) (*url.URL, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse image factory URL %q: %w", rawURL, err)
	}

	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("image factory URL %q is not absolute", rawURL)
	}

	return parsed, nil
}

// endpoint is the image factory serving a Talos version, with the credentials Omni holds for it.
type endpoint struct {
	username   string
	password   string
	baseURL    string
	pxeBaseURL string
}

// resolveEndpoint returns the image factory that serves the given Talos version.
func resolveEndpoint(ctx context.Context, st state.State, talosVersion string) (endpoint, error) {
	config, err := safe.ReaderGetByID[*omni.FeaturesConfig](ctx, st, omni.FeaturesConfigID)
	if err != nil {
		return endpoint{}, fmt.Errorf("failed to get features config: %w", err)
	}

	spec := config.TypedSpec().Value
	baseURL, pxeBaseURL := spec.GetImageFactoryBaseUrl(), spec.GetImageFactoryPxeBaseUrl()

	if secondary := spec.GetSecondaryImageFactoryBaseUrl(); secondary != "" {
		if talosVersion == "" {
			talosVersion = constants.DefaultTalosVersion
		}

		var recordedURL string

		if recordedURL, err = recordedFactoryURLForVersion(ctx, st, talosVersion); err != nil {
			return endpoint{}, fmt.Errorf("failed to resolve the factory of Talos version %q: %w", talosVersion, err)
		}

		// A version Omni doesn't know about counts as primary, the same fallback ForTalosVersion makes.
		if recordedURL != "" && recordedURL == normalizeFactoryURL(secondary) {
			baseURL, pxeBaseURL = secondary, spec.GetSecondaryImageFactoryPxeBaseUrl()
		}
	}

	if baseURL == "" {
		baseURL = constants.ImageFactoryBaseURL
	}

	// Check the base URL before deriving anything from it, so the error names what Omni was given rather
	// than the mangled host derivePXEBaseURL would build out of it.
	//
	// A configured PXE URL is deliberately left to the PXE asset build. Resolving also serves callers
	// that only download images, and a malformed PXE URL is no reason to deny them.
	parsedBaseURL, err := parseFactoryURL(baseURL)
	if err != nil {
		return endpoint{}, err
	}

	if pxeBaseURL == "" {
		pxeBaseURL = derivePXEBaseURL(parsedBaseURL)
	}

	username, password, err := credentialsAllowingDenied(ctx, st, baseURL)
	if err != nil {
		return endpoint{}, err
	}

	return endpoint{
		username:   username,
		password:   password,
		baseURL:    baseURL,
		pxeBaseURL: pxeBaseURL,
	}, nil
}

// derivePXEBaseURL builds the PXE endpoint for a factory whose PXE URL Omni does not report, by prefixing
// the host with "pxe.". Omni derives an unconfigured PXE URL the same way.
//
// The caller has already checked that baseURL is absolute, which is what makes the result absolute too.
func derivePXEBaseURL(baseURL *url.URL) string {
	derived := *baseURL
	derived.Host = "pxe." + derived.Host

	return derived.String()
}
