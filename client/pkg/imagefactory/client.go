// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package imagefactory

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/siderolabs/image-factory/pkg/client"
	"github.com/siderolabs/image-factory/pkg/schematic"
)

// requestTimeout caps every call to the image factory. Without it, the factory's HTTP client has
// no internal timeout, so a stuck connection would pin a controller reconcile slot indefinitely.
// 30 minutes as the scan report request can be particularly slow.
const requestTimeout = 30 * time.Minute

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
	host    string
	url     string
}

// NewClient creates a new image factory client.
func NewClient(imageFactoryBaseURL, username, password string) (*Client, error) {
	sniffer := &serverSnifferTransport{wrapped: http.DefaultTransport}

	clientOptions := []client.Option{
		client.WithClient(http.Client{Transport: sniffer, Timeout: requestTimeout}),
	}

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
		Client:  factoryClient,
		host:    baseURL.Host,
		url:     imageFactoryBaseURL,
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

// URL returns the base URL of the image factory client, as it was configured.
func (cli *Client) URL() string {
	if cli == nil {
		return ""
	}

	return cli.url
}

// CachedIsEnterprise reports whether the connected image factory is an Enterprise instance.
// The value is detected from the Server response header of the first successful HTTP response
// and cached; it returns false until at least one response has been received.
func (cli *Client) CachedIsEnterprise() bool {
	return cli.sniffer.isEnterprise.Load()
}

// EnsureSchematic uploads the given schematic to the image factory and returns its ID
// along with the normalized schematic as the factory persisted it.
//
// The factory deduplicates by content: if the same schematic was uploaded before, it returns
// the existing ID without creating a new one.
func (cli *Client) EnsureSchematic(ctx context.Context, inputSchematic schematic.Schematic) (string, *schematic.Schematic, error) {
	if !cli.sniffer.detected.Load() {
		// if not loaded yet, make a request to detect the factory version and whether it's Enterprise or not
		if _, err := cli.Versions(ctx); err != nil {
			return "", nil, fmt.Errorf("failed to detect image factory version: %w", err)
		}
	}

	// drop the owner from the schematic before sending it to the factory for the community version
	if !cli.CachedIsEnterprise() {
		inputSchematic.Owner = ""
	}

	id, data, err := cli.SchematicCreate(ctx, inputSchematic)
	if err != nil {
		return "", nil, fmt.Errorf("failed to ensure schematic: %w", err)
	}

	return id, data, nil
}
