// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package imagefactory

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/image-factory/pkg/client"
	"github.com/siderolabs/image-factory/pkg/schematic"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// FactoryClient is the contract Omni controllers rely on for interacting with a single image factory.
type FactoryClient interface { //nolint:interfacebloat
	EnsureSchematic(ctx context.Context, inputSchematic schematic.Schematic) (string, *schematic.Schematic, error)
	SchematicGet(ctx context.Context, id string) (*schematic.Schematic, error)
	Host() string
	URL() string
	OverlaysVersions(context.Context, string) ([]client.OverlayInfo, error)
	Versions(context.Context) ([]string, error)
	ExtensionsVersions(context.Context, string) ([]client.ExtensionInfo, error)
	CachedIsEnterprise() bool
	TalosctlList(ctx context.Context, talosVersion string) ([]string, error)
	DownloadToken(ctx context.Context, ttl time.Duration) (string, error)
	TokenCreate(ctx context.Context, opts client.TokenCreateOptions) (id, token string, err error)
	ReportsClient
}

// ReportsClient is the part of the contract covering a factory's security reports: the vulnerability
// scan of a schematic, its SBOM bundle, and the VEX document of a Talos version.
type ReportsClient interface {
	ScanReport(ctx context.Context, schematicID, talosVersion, arch, filename string) ([]byte, error)
	SPDXBundle(ctx context.Context, schematicID, talosVersion, arch string) ([]byte, error)
	VEXDocument(ctx context.Context, talosVersion string) ([]byte, error)
}

// Clients holds the configured image factory clients: a required primary and an optional secondary.
//
// Omni supports up to two image factories. The primary factory is always configured and is used for
// all new schematic/install-image work; the secondary factory is an optional peer that existing
// machines may still belong to. Callers that operate on a specific machine should route through
// ForURL, using the factory URL tracked on the machine's resources.
type Clients struct {
	st        state.State
	primary   FactoryClient
	secondary FactoryClient
}

// NewClients creates a new image factory client set. The secondary client may be nil when no
// secondary factory is configured.
func NewClients(st state.State, primary FactoryClient) *Clients {
	res := &Clients{
		st:      st,
		primary: primary,
	}

	return res
}

// NewClientsFromState creates a new image factory client set from the state.
func NewClientsFromState(ctx context.Context, st state.State) (*Clients, error) {
	config, err := safe.ReaderGetByID[*omni.FeaturesConfig](ctx, st, omni.FeaturesConfigID)
	if err != nil {
		return nil, err
	}

	baseURL := config.TypedSpec().Value.ImageFactoryBaseUrl
	if baseURL == "" {
		baseURL = constants.ImageFactoryBaseURL
	}

	auth, err := credentialsAllowingDenied(ctx, st, baseURL)
	if err != nil {
		return nil, err
	}

	primaryClient, err := NewClient(baseURL, auth)
	if err != nil {
		return nil, err
	}

	clients := NewClients(st, primaryClient)

	if config.TypedSpec().Value.SecondaryImageFactoryBaseUrl != "" {
		secondaryAuth, err := credentialsAllowingDenied(ctx, st, config.TypedSpec().Value.SecondaryImageFactoryBaseUrl)
		if err != nil {
			return nil, err
		}

		secondaryClient, err := NewClient(config.TypedSpec().Value.SecondaryImageFactoryBaseUrl, secondaryAuth)
		if err != nil {
			return nil, err
		}

		clients.SetSecondary(secondaryClient)
	}

	return clients, nil
}

// credentialsAllowingDenied reads the credentials for a factory, treating a denied read as "no credentials".
//
// An Omni that predates ImageFactoryAuth, or a caller whose role cannot read it, denies the read outright.
// Callers that only need to reach a factory serving assets anonymously should still get there, and one that
// does need credentials fails on its own with a 401 naming the factory.
func credentialsAllowingDenied(ctx context.Context, st state.State, factoryURL string) (Auth, error) {
	auth, err := Credentials(ctx, st, factoryURL)
	if err != nil && status.Code(err) != codes.PermissionDenied {
		return Auth{}, err
	}

	return auth, nil
}

// Credentials returns what Omni authenticates to the image factory at the given URL with, or a zero Auth
// when that factory has none configured.
func Credentials(ctx context.Context, st state.State, factoryURL string) (Auth, error) {
	auth, err := safe.ReaderGetByID[*omni.ImageFactoryAuth](ctx, st, NormalizeFactoryURL(factoryURL))
	if err != nil {
		if state.IsNotFoundError(err) {
			return Auth{}, nil
		}

		return Auth{}, fmt.Errorf("failed to get image factory auth: %w", err)
	}

	spec := auth.TypedSpec().Value

	creds := Auth{Username: spec.GetUsername(), Password: spec.GetPassword()}

	if token := spec.GetApiToken(); token != "" {
		creds.TokenSource = func() string { return token }
	}

	return creds, nil
}

// SetSecondary configures the secondary image factory client.
func (c *Clients) SetSecondary(secondary FactoryClient) {
	c.secondary = secondary
}

// NormalizeFactoryURL strips a trailing slash so that factory URLs coming from different sources
// (Omni's own configuration, a TalosVersion resource, a client request) compare equal.
func NormalizeFactoryURL(url string) string {
	return strings.TrimRight(url, "/")
}

// ForURL returns the image factory client configured for the given URL, or nil when no client is configured for that URL.
//
// The URL may come straight from a client request, so it is normalized before comparing.
func (c *Clients) ForURL(url string) FactoryClient {
	url = NormalizeFactoryURL(url)

	clients := []FactoryClient{c.primary}
	if c.secondary != nil {
		clients = append(clients, c.secondary)
	}

	for _, client := range clients {
		if client.URL() == url {
			return client
		}
	}

	return nil
}

// ForHost returns the image factory client configured for the given host, or nil when no client is configured for that host.
//
// Hosts need no normalization: they are derived from a parsed URL and never carry a trailing slash.
func (c *Clients) ForHost(host string) FactoryClient {
	clients := []FactoryClient{c.primary}
	if c.secondary != nil {
		clients = append(clients, c.secondary)
	}

	for _, client := range clients {
		if client.Host() == host {
			return client
		}
	}

	return nil
}

// recordedFactoryURLForVersion returns the URL of the factory Omni recorded for the given Talos version,
// or an empty string when the version is unknown or carries no URL.
func recordedFactoryURLForVersion(ctx context.Context, st state.State, talosVersion string) (string, error) {
	version, err := safe.ReaderGetByID[*omni.TalosVersion](ctx, st, strings.TrimLeft(talosVersion, "v"))
	if err != nil {
		if state.IsNotFoundError(err) {
			return "", nil
		}

		return "", err
	}

	// The recorded URL may predate the canonicalization in NewClient, so normalize both sides.
	return NormalizeFactoryURL(version.TypedSpec().Value.GetImageFactoryUrl()), nil
}

// ForTalosVersion returns the image factory client configured for the given Talos version, falling back to the primary client when no version is found or the version does not specify a factory URL.
func (c *Clients) ForTalosVersion(ctx context.Context, v string) (FactoryClient, error) {
	recordedURL, err := recordedFactoryURLForVersion(ctx, c.st, v)
	if err != nil {
		return nil, err
	}

	if recordedURL == "" {
		return c.Primary(), nil
	}

	clients := []FactoryClient{c.primary}
	if c.secondary != nil {
		clients = append(clients, c.secondary)
	}

	for _, client := range clients {
		if NormalizeFactoryURL(client.URL()) == recordedURL {
			return client, nil
		}
	}

	return c.Primary(), nil
}

// Primary returns the primary image factory client.
func (c *Clients) Primary() FactoryClient {
	if c == nil {
		return nil
	}

	return c.primary
}

// Secondary returns the secondary image factory client and true, or nil and false when no secondary
// factory is configured.
func (c *Clients) Secondary() (FactoryClient, bool) {
	if c == nil || c.secondary == nil {
		return nil, false
	}

	return c.secondary, true
}
