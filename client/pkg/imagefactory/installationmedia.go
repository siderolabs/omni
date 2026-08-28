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
	"time"

	"github.com/blang/semver/v4"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/image-factory/pkg/client"
	"github.com/siderolabs/talos/pkg/machinery/platforms"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// InstallationMediaKind selects one of the kinds the image factory serves.
type InstallationMediaKind string

const (
	// InstallationMediaKindPXE is an iPXE script, served from the factory's PXE endpoint.
	InstallationMediaKindPXE InstallationMediaKind = "pxe"

	// InstallationMediaKindISO is a bootable ISO image.
	InstallationMediaKindISO InstallationMediaKind = "iso"

	// InstallationMediaKindDisk is a bootable disk image, in the disk format named by MediaSpec.Format.
	InstallationMediaKindDisk InstallationMediaKind = "disk"
)

// ErrInvalidInput marks the arguments a caller got wrong, as opposed to a misconfigured image factory.
// A server turning these into a response should answer InvalidArgument for them and not for the rest.
var ErrInvalidInput = errors.New("invalid installation media request")

// MediaSpec identifies an installation medium, without saying how the image factory spells it.
type MediaSpec struct {
	Kind InstallationMediaKind

	// Platform is the Talos platform, such as "metal" or "nocloud".
	Platform string

	// Architecture is the machine architecture, such as "amd64" or "arm64".
	Architecture string

	// Format is the disk image format of the disk kind, such as "raw.xz", "qcow2" or "qcow2.gz". It is
	// unused by every other kind, whose format the kind itself implies.
	Format string

	// SecureBoot selects the secure boot variant.
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

// Validate reports whether the spec identifies a medium the image factory can serve.
func (s MediaSpec) Validate() error {
	switch s.Kind {
	case InstallationMediaKindPXE, InstallationMediaKindISO, InstallationMediaKindDisk:
	case "":
		return fmt.Errorf("%w: installation media kind is not set", ErrInvalidInput)
	default:
		return fmt.Errorf("%w: unknown installation media kind %q", ErrInvalidInput, s.Kind)
	}

	if !validPathSegment(s.Platform) {
		return fmt.Errorf("%w: invalid platform %q", ErrInvalidInput, s.Platform)
	}

	if !validPathSegment(s.Architecture) {
		return fmt.Errorf("%w: invalid architecture %q", ErrInvalidInput, s.Architecture)
	}

	if s.Kind == InstallationMediaKindDisk && !validPathSegment(s.Format) {
		return fmt.Errorf("%w: invalid disk image format %q", ErrInvalidInput, s.Format)
	}

	return nil
}

// Filename returns the filename the image factory serves this medium under.
func (s MediaSpec) Filename() string {
	platform := platforms.Platform{Name: s.Platform}

	switch s.Kind {
	case InstallationMediaKindPXE:
		if s.SecureBoot {
			return platform.SecureBootPXEScriptPath(s.Architecture)
		}

		return platform.PXEScriptPath(s.Architecture)
	case InstallationMediaKindISO:
		if s.SecureBoot {
			return platform.SecureBootISOPath(s.Architecture)
		}

		return platform.ISOPath(s.Architecture)
	case InstallationMediaKindDisk:
		if s.SecureBoot {
			return platform.SecureBootDiskImagePath(s.Architecture, s.Format)
		}

		return platform.DiskImagePath(s.Architecture, s.Format)
	}

	return ""
}

// InstallationMedia is a fetchable installation medium.
//
// Fetch URL, sending Headers when they are not empty. The URL is opaque and can carry secrets, whether
// as credentials or as a factory token, so it does not belong in a log line.
type InstallationMedia struct {
	// Headers must be sent with the request fetching URL. Empty when the factory needs no
	// authentication, or when it travels inside the URL instead.
	Headers http.Header

	// ExpiresAt is when URL stops working, for a caller that does not perform the fetch itself and has
	// to know how long the URL it passed on stays good. Zero when the URL does not expire: an anonymous
	// factory, or one authenticated with credentials rather than a token.
	//
	// It is an estimate. The factory confirms only that it granted the lifetime Omni asked for, never the
	// instant it started counting, and it allows another 30s of clock leeway when verifying. Treat it as
	// the earliest moment the URL may stop working.
	ExpiresAt time.Time

	// URL of the installation medium.
	URL string

	// SchematicID is the ID the image factory gave the schematic this medium is built from.
	SchematicID string

	// StorageKey identifies this medium, for callers that store what they download. It changes only when
	// the medium itself does, so it survives a credential rotation, which the URL does not: a name derived
	// from the URL would change with the credentials in it and orphan whatever was stored under the old
	// one.
	//
	// Treat it as opaque. Omni may change how it is derived, which costs a caller one re-download of the
	// media it already holds.
	StorageKey string

	// ImageFactoryHost is the host serving the medium, the same one in the URL. Purely informative: to
	// fetch it, use the URL and the headers.
	ImageFactoryHost string
}

// String returns a representation that is safe to log.
//
// The URL is left out on purpose: it can carry credentials as userinfo or a token in the query, so a
// logged InstallationMedia would leak them. What remains identifies the medium just as well. This covers %v and
// zap.Any, which prefer a Stringer, but not reflection or json.Marshal.
func (a InstallationMedia) String() string {
	var expires string

	if !a.ExpiresAt.IsZero() {
		expires = ", expires: " + a.ExpiresAt.Format(time.RFC3339)
	}

	return fmt.Sprintf("InstallationMedia{host: %q, schematic: %q, key: %q%s}", a.ImageFactoryHost, a.SchematicID, a.StorageKey, expires)
}

// storageKey derives the identity of a medium from everything that decides its content: the factory
// serving it, the schematic, the Talos version and the name the factory serves it under.
//
// Deliberately not the URL, which also carries the credentials and so changes when they rotate.
//
// **Important:** New inputs must be appended only when they are set, so that every medium hashed
// before the input existed keeps its key. A changed key makes the providers
// re-download everything they already store.
func storageKey(factoryBaseURL, pathPrefix, schematicID, version, filename string) string {
	digest := sha256.Sum256([]byte(strings.Join([]string{
		normalizeFactoryURL(factoryBaseURL), pathPrefix, schematicID, version, filename,
	}, "\x00")))

	return hex.EncodeToString(digest[:])
}

// ResolveOption configures how a medium is resolved. With no options, the download is authenticated with
// the basic auth credentials Omni holds.
type ResolveOption func(*resolveOptions)

type resolveOptions struct {
	downloadTokens *DownloadTokenOptions
}

// DownloadTokenOptions is what Omni needs to authenticate a download with a token issued by the factory
// rather than with the basic auth credentials it holds.
type DownloadTokenOptions struct {
	Factories *Clients
	Logger    *zap.Logger
	TTL       time.Duration
}

// logger returns the logger the caller supplied, or a no-op one.
func (o *DownloadTokenOptions) logger() *zap.Logger {
	if o == nil || o.Logger == nil {
		return zap.NewNop()
	}

	return o.Logger
}

// defaultDownloadTokenTTL is what Omni asks for when the caller asks for nothing.
const defaultDownloadTokenTTL = 5 * time.Minute

// downloadTokenRequestTimeout bounds the token request on its own, since the factory client's timeout is
// sized for the slowest thing it does.
const downloadTokenRequestTimeout = 30 * time.Second

// WithDownloadTokens authenticates an image download with a short-lived token issued by the factory
// instead of the basic auth credentials Omni holds.
func WithDownloadTokens(opts DownloadTokenOptions) ResolveOption {
	return func(o *resolveOptions) {
		o.downloadTokens = &opts
	}
}

// requestDownloadToken returns a token authenticating a download from the factory at baseURL and the
// moment it stops working.
func requestDownloadToken(ctx context.Context, baseURL string, opts *DownloadTokenOptions) (string, time.Time, bool, error) {
	// A caller that cannot ask for a token, such as the provider-side fallback against an older Omni,
	// passes nothing.
	if opts == nil || opts.Factories == nil {
		return "", time.Time{}, false, nil
	}

	logger := opts.logger()

	factory := opts.Factories.ForURL(baseURL)
	if factory == nil {
		logger.Warn("no image factory client is configured for the factory serving this medium, so no download token was requested",
			zap.String("factory_url", baseURL))

		return "", time.Time{}, false, nil
	}

	ttl := opts.TTL
	if ttl == 0 {
		ttl = defaultDownloadTokenTTL
	}

	tokenCtx, cancel := context.WithTimeout(ctx, downloadTokenRequestTimeout)
	defer cancel()

	requestedAt := time.Now()
	token, err := factory.DownloadToken(tokenCtx, ttl)

	switch {
	case err == nil && token == "":
		logger.Warn("the image factory returned an empty download token",
			zap.String("factory_url", baseURL))

		return "", time.Time{}, false, nil
	case err == nil:
		return token, requestedAt.Add(ttl), true, nil
	case client.IsHTTPErrorCode(err, http.StatusNotFound), client.IsHTTPErrorCode(err, http.StatusMethodNotAllowed):
		logger.Debug("the image factory does not issue download tokens",
			zap.String("factory_url", baseURL), zap.Error(err))

		return "", time.Time{}, false, nil
	case client.IsInvalidSchematicError(err):
		if opts.TTL != 0 {
			logger.Warn("the image factory refused the requested download token lifetime",
				zap.String("factory_url", baseURL), zap.Duration("ttl", ttl), zap.Error(err))

			return "", time.Time{}, false, fmt.Errorf("%w: the image factory refused a download token lifetime of %s: %w", ErrInvalidInput, ttl, err)
		}

		logger.Warn("the image factory refused the default download token lifetime",
			zap.String("factory_url", baseURL), zap.Duration("ttl", ttl), zap.Error(err))

		return "", time.Time{}, false, nil
	case ctx.Err() != nil:
		return "", time.Time{}, false, fmt.Errorf("failed to request an image factory download token: %w: %w", ctx.Err(), err)
	default:
		logger.Warn("failed to obtain an image factory download token",
			zap.String("factory_url", baseURL), zap.Error(err))

		return "", time.Time{}, false, nil
	}
}

// ResolveInstallationMedia returns the installation medium the given schematic produces, served by whichever image
// factory Omni uses for the given Talos version. An empty version means the default Talos version.
//
// Set standalone when the fetch cannot carry headers, such as a hypervisor download API handed a bare
// URL: any authentication then travels inside the URL and Headers comes back empty. The PXE kind is
// always resolved that way, since the firmware fetching the script cannot send headers either.
//
// Headers can come back empty without standalone too, since a factory issuing download tokens
// authenticates the download inside the URL for every caller. Send Headers whenever it is not empty,
// instead of deciding from what was asked for.
func ResolveInstallationMedia(
	ctx context.Context, st state.State, talosVersion string, spec MediaSpec, schematicID string, standalone bool, opts ...ResolveOption,
) (InstallationMedia, error) {
	var options resolveOptions

	for _, opt := range opts {
		opt(&options)
	}

	if err := spec.Validate(); err != nil {
		return InstallationMedia{}, err
	}

	if options.downloadTokens != nil && options.downloadTokens.TTL < 0 {
		return InstallationMedia{}, fmt.Errorf("%w: download token lifetime %s is negative", ErrInvalidInput, options.downloadTokens.TTL)
	}

	if talosVersion == "" {
		talosVersion = constants.DefaultTalosVersion
	}

	if _, err := semver.ParseTolerant(talosVersion); err != nil {
		return InstallationMedia{}, fmt.Errorf("%w: invalid Talos version %q: %w", ErrInvalidInput, talosVersion, err)
	}

	version := "v" + strings.TrimLeft(talosVersion, "v")

	if !validPathSegment(schematicID) {
		return InstallationMedia{}, fmt.Errorf("%w: invalid schematic ID %q", ErrInvalidInput, schematicID)
	}

	resolved, err := resolveEndpoint(ctx, st, talosVersion)
	if err != nil {
		return InstallationMedia{}, err
	}

	baseURL, pathPrefix := resolved.baseURL, "image"
	if spec.Kind == InstallationMediaKindPXE {
		baseURL, pathPrefix = resolved.pxeBaseURL, "pxe"
	}

	parsed, err := parseFactoryURL(baseURL)
	if err != nil {
		return InstallationMedia{}, err
	}

	filename := spec.Filename()
	mediaURL := parsed.JoinPath(pathPrefix, schematicID, version, filename)

	media := InstallationMedia{
		SchematicID:      schematicID,
		StorageKey:       storageKey(baseURL, pathPrefix, schematicID, version, filename),
		ImageFactoryHost: mediaURL.Host,
	}

	switch {
	case resolved.username == "" || resolved.password == "":
		// The factory serves this anonymously, so there is nothing to place.
	case spec.Kind == InstallationMediaKindPXE:
		mediaURL.User = url.UserPassword(resolved.username, resolved.password)
	default:
		token, expiresAt, ok, err := requestDownloadToken(ctx, resolved.baseURL, options.downloadTokens)

		switch {
		case err != nil:
			return InstallationMedia{}, err
		case ok:
			query := mediaURL.Query()
			query.Set("token", token)
			mediaURL.RawQuery = query.Encode()

			media.ExpiresAt = expiresAt
		case standalone:
			mediaURL.User = url.UserPassword(resolved.username, resolved.password)

			options.downloadTokens.logger().Debug("authenticating the installation media download with credentials in the URL",
				zap.String("factory_url", resolved.baseURL))
		default:
			media.Headers = http.Header{
				"Authorization": []string{"Basic " + base64.StdEncoding.EncodeToString([]byte(resolved.username+":"+resolved.password))},
			}

			options.downloadTokens.logger().Debug("authenticating the installation media download with a credentials header",
				zap.String("factory_url", resolved.baseURL))
		}
	}

	media.URL = mediaURL.String()

	return media, nil
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
	// A configured PXE URL is deliberately left to the PXE build. Resolving also serves callers
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
