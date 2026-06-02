// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package imagefactory

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/image-factory/pkg/client"
	"github.com/siderolabs/image-factory/pkg/schematic"
)

// serverSnifferTransport wraps an http.RoundTripper and records whether the image factory
// identifies itself as an Enterprise instance via the Server response header.
// It captures the header from the first successful response so no extra requests are needed.
type serverSnifferTransport struct {
	wrapped      http.RoundTripper
	detected     atomic.Bool
	isEnterprise atomic.Bool
}

func (t *serverSnifferTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.wrapped.RoundTrip(req)
	if err == nil && t.detected.CompareAndSwap(false, true) {
		t.isEnterprise.Store(strings.Contains(resp.Header.Get("Server"), "Enterprise"))
	}

	return resp, err
}

// Client is the image factory client.
type Client struct {
	*client.Client

	sniffer *serverSnifferTransport
	state   state.State
	host    string
}

// NewClient creates a new image factory client.
func NewClient(omniState state.State, imageFactoryBaseURL, username, password string) (*Client, error) {
	sniffer := &serverSnifferTransport{wrapped: http.DefaultTransport}

	var clientOptions []client.Option

	clientOptions = append(clientOptions, client.WithClient(http.Client{Transport: sniffer}))

	if username != "" && password != "" {
		clientOptions = append(clientOptions, client.WithBasicAuth(username, password))
	}

	factoryClient, err := client.New(imageFactoryBaseURL, clientOptions...)
	if err != nil {
		return nil, err
	}

	baseURL, err := url.Parse(imageFactoryBaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse image factory base URL %q: %w", imageFactoryBaseURL, err)
	}

	return &Client{
		state:   omniState,
		Client:  factoryClient,
		host:    baseURL.Host,
		sniffer: sniffer,
	}, nil
}

// Host returns the host of the image factory client.
func (cli *Client) Host() string {
	if cli == nil {
		return ""
	}

	return cli.host
}

// CachedIsEnterprise reports whether the connected image factory is an Enterprise instance.
// The value is detected from the Server response header of the first successful HTTP response
// and cached; it returns false until at least one response has been received.
func (cli *Client) CachedIsEnterprise() bool {
	return cli.sniffer.isEnterprise.Load()
}

// EnsuredSchematic contains information on the ensured schematics.
type EnsuredSchematic struct {
	Data    *schematic.Schematic
	FullID  string
	PlainID string
}

// EnsureSchematic ensures that the given schematic exists in the image factory.
//
// It ensures two schematics: one with the full content of the schematic.Schematic, and another with only the system extensions.
//
// We do not call the image factory for the schematics that are already known to (cached by) Omni.
func (cli *Client) EnsureSchematic(ctx context.Context, inputSchematic schematic.Schematic) (EnsuredSchematic, error) {
	fullSchematicID, data, err := cli.SchematicCreate(ctx, inputSchematic)
	if err != nil {
		return EnsuredSchematic{}, fmt.Errorf("failed to ensure single schematic: %w", err)
	}

	plainSchematic := schematic.Schematic{
		Customization: schematic.Customization{
			SystemExtensions: schematic.SystemExtensions{
				OfficialExtensions: inputSchematic.Customization.SystemExtensions.OfficialExtensions,
			},
		},
	}

	plainSchematicID, err := plainSchematic.ID()
	if err != nil {
		return EnsuredSchematic{}, fmt.Errorf("failed to generate plain schematic ID: %w", err)
	}

	if plainSchematicID != fullSchematicID {
		plainSchematicID, _, err = cli.SchematicCreate(ctx, plainSchematic)
		if err != nil {
			return EnsuredSchematic{}, fmt.Errorf("failed to ensure plain schematic: %w", err)
		}
	}

	return EnsuredSchematic{
		FullID:  fullSchematicID,
		PlainID: plainSchematicID,
		Data:    data,
	}, nil
}
