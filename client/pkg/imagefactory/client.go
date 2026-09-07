// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package imagefactory

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
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
	isEnterprise atomic.Bool

	// detectMu serializes the probe in detectEnterprise so concurrent callers issue a single request.
	// It must never be held across a RoundTrip, which takes mu.
	detectMu sync.Mutex

	mu       sync.Mutex
	detected bool
}

func (t *serverSnifferTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.wrapped.RoundTrip(req)

	t.mu.Lock()

	if err == nil && !t.detected {
		t.detected = true

		t.isEnterprise.Store(strings.Contains(resp.Header.Get("Server"), "Enterprise"))
	}

	t.mu.Unlock()

	return resp, err
}

func (t *serverSnifferTransport) isDetected() bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.detected
}

// Client is the image factory client.
type Client struct {
	*client.Client

	sniffer *serverSnifferTransport
	host    string
	url     string
}

// Auth is what Omni authenticates to an image factory with: an API token, or basic auth credentials.
// The zero value is anonymous access, for a factory that requires no authentication.
//
// The token is looked up on every request through TokenSource, so that a rotated token is used
// without rebuilding the client. Token is the static alternative, for callers that hold the token
// itself, such as a client built from the state Omni keeps it in.
type Auth struct {
	TokenSource func() string
	Username    string
	Password    string
}

// IsZero reports whether no credential is set.
func (a Auth) IsZero() bool {
	return a.TokenSource == nil && (a.Username == "" || a.Password == "")
}

// NewClient creates a new image factory client.
//
// The base URL is canonicalized by stripping any trailing slash, so that the URL reported by
// [Client.URL] can be compared to a factory URL from any other source (a configured factory, a
// TalosVersion resource, a client request) without each comparison having to normalize first.
func NewClient(imageFactoryBaseURL string, auth Auth) (*Client, error) {
	imageFactoryBaseURL = NormalizeFactoryURL(imageFactoryBaseURL)

	sniffer := &serverSnifferTransport{wrapped: http.DefaultTransport}

	clientOptions := []client.Option{
		client.WithClient(http.Client{Transport: sniffer, Timeout: requestTimeout}),
	}

	switch {
	case auth.TokenSource != nil:
		clientOptions = append(clientOptions, client.WithTokenSource(func(context.Context) (string, error) {
			token := auth.TokenSource()
			if token == "" {
				// The factory client sends no header for an empty token, and the resulting 401 would
				// not point at the token.
				return "", errors.New("the image factory token is empty")
			}

			return token, nil
		}))
	case auth.Username != "" && auth.Password != "":
		clientOptions = append(clientOptions, client.WithBasicAuth(auth.Username, auth.Password))
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

// URL returns the canonical base URL of the image factory client: as it was configured, with any
// trailing slash stripped.
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

// detectEnterprise makes sure the factory's server type has been detected, issuing a single probe
// request if it has not. Concurrent callers are serialized on detectMu so only one probe is made.
//
// It must not hold sniffer.mu while probing: the request's RoundTrip takes that mutex itself.
func (cli *Client) detectEnterprise(ctx context.Context) error {
	cli.sniffer.detectMu.Lock()
	defer cli.sniffer.detectMu.Unlock()

	// if we already detected the server type, no need to make a request
	if cli.sniffer.isDetected() {
		return nil
	}

	// make a request to detect the server type
	_, err := cli.Versions(ctx)

	return err
}

// EnsureSchematic uploads the given schematic to the image factory and returns its ID
// along with the normalized schematic as the factory persisted it.
//
// The factory deduplicates by content: if the same schematic was uploaded before, it returns
// the existing ID without creating a new one.
func (cli *Client) EnsureSchematic(ctx context.Context, inputSchematic schematic.Schematic) (string, *schematic.Schematic, error) {
	if err := cli.detectEnterprise(ctx); err != nil {
		return "", nil, fmt.Errorf("failed to detect image factory enterprise status: %w", err)
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
