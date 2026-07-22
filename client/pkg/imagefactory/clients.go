// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package imagefactory

import (
	"context"
	"strings"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/image-factory/pkg/client"
	"github.com/siderolabs/image-factory/pkg/schematic"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/virtual"
)

// FactoryClient is the contract Omni controllers rely on for interacting with a single image factory.
type FactoryClient interface {
	EnsureSchematic(ctx context.Context, inputSchematic schematic.Schematic) (string, *schematic.Schematic, error)
	SchematicGet(ctx context.Context, id string) (*schematic.Schematic, error)
	Host() string
	URL() string
	OverlaysVersions(context.Context, string) ([]client.OverlayInfo, error)
	Versions(context.Context) ([]string, error)
	ExtensionsVersions(context.Context, string) ([]client.ExtensionInfo, error)
	CachedIsEnterprise() bool
	TalosctlList(ctx context.Context, talosVersion string) ([]string, error)
	ScanReport(ctx context.Context, schematicID, talosVersion, arch, filename string) ([]byte, error)
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

	username, password, err := getFactoryCreds(ctx, st, config.TypedSpec().Value.ImageFactoryBaseUrl)
	if err != nil {
		return nil, err
	}

	primaryClient, err := NewClient(config.TypedSpec().Value.ImageFactoryBaseUrl, username, password)
	if err != nil {
		return nil, err
	}

	clients := NewClients(st, primaryClient)

	if config.TypedSpec().Value.SecondaryImageFactoryBaseUrl != "" {
		secondaryUsername, secondaryPassword, err := getFactoryCreds(ctx, st, config.TypedSpec().Value.SecondaryImageFactoryBaseUrl)
		if err != nil {
			return nil, err
		}

		secondaryClient, err := NewClient(config.TypedSpec().Value.SecondaryImageFactoryBaseUrl, secondaryUsername, secondaryPassword)
		if err != nil {
			return nil, err
		}

		clients.SetSecondary(secondaryClient)
	}

	return clients, nil
}

func getFactoryCreds(ctx context.Context, st state.State, id string) (string, string, error) {
	auth, err := safe.ReaderGetByID[*virtual.ImageFactoryAuth](ctx, st, id)
	if err != nil && !state.IsNotFoundError(err) {
		return "", "", err
	}

	if auth == nil {
		return "", "", nil
	}

	return auth.TypedSpec().Value.Username, auth.TypedSpec().Value.Password, nil
}

// SetSecondary configures the secondary image factory client.
func (c *Clients) SetSecondary(secondary FactoryClient) {
	c.secondary = secondary
}

// ForURL returns the image factory client configured for the given URL, or nil when no client is configured for that URL.
func (c *Clients) ForURL(url string) FactoryClient {
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

// ForTalosVersion returns the image factory client configured for the given Talos version, falling back to the primary client when no version is found or the version does not specify a factory URL.
func (c *Clients) ForTalosVersion(ctx context.Context, v string) (FactoryClient, error) {
	version, err := safe.ReaderGetByID[*omni.TalosVersion](ctx, c.st, strings.TrimLeft(v, "v"))
	if err != nil && !state.IsNotFoundError(err) {
		return nil, err
	}

	if version == nil {
		return c.Primary(), nil
	}

	clients := []FactoryClient{c.primary}
	if c.secondary != nil {
		clients = append(clients, c.secondary)
	}

	for _, client := range clients {
		if client.URL() == version.TypedSpec().Value.ImageFactoryUrl {
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
