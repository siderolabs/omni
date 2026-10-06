// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package k8sproxy_test

import (
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/transport"

	"github.com/siderolabs/omni/internal/backend/k8sproxy"
)

// allowedHeaders carries every header the apiserver is sent, so that dropping one of them fails here.
var allowedHeaders = http.Header{
	"Accept":                    {"application/vnd.kubernetes.protobuf"},
	"Accept-Encoding":           {"gzip"},
	"Content-Type":              {"application/apply-patch+yaml"},
	"If-Modified-Since":         {"Mon, 06 Oct 2026 00:00:00 GMT"},
	"If-None-Match":             {"some-etag"},
	"User-Agent":                {"kubectl/v1.35.9"},
	"Connection":                {"Upgrade"},
	"Origin":                    {"https://example.com"},
	"Sec-Websocket-Key":         {"some-key"},
	"Sec-Websocket-Protocol":    {"v5.channel.k8s.io"},
	"Sec-Websocket-Version":     {"13"},
	"Upgrade":                   {"websocket"},
	"X-Stream-Protocol-Version": {"v4.channel.k8s.io"},
}

func TestForwardHeaders(t *testing.T) {
	for _, test := range []struct {
		header   http.Header
		expected http.Header
		name     string
	}{
		{
			name:     "a request keeps everything it needs",
			header:   allowedHeaders,
			expected: allowedHeaders,
		},
		{
			name: "the caller decides nothing about who they are",
			header: http.Header{
				"Authorization": {"Bearer some-token"}, "Impersonate-User": {"someone-else"}, "Impersonate-Group": {"some-group"},
				"Impersonate-Uid": {"some-uid"}, "Impersonate-Extra-Scopes": {"some-value"},
			},
			expected: http.Header{},
		},
		{
			name:     "a cluster can name its own front proxy headers, so they are dropped by default",
			header:   http.Header{"X-Remote-User": {"someone-else"}, "X-Remote-Group": {"some-group"}, "X-Custom-Configured-User": {"someone-else"}},
			expected: http.Header{},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, k8sproxy.ForwardHeaders(test.header, nil))
		})
	}
}

// TestForwardedImpersonation checks that the apiserver is told who the caller is, whatever the request
// carries.
func TestForwardedImpersonation(t *testing.T) {
	var upstream http.Header

	apiserver := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
		upstream = req.Header.Clone()
	}))

	t.Cleanup(apiserver.Close)

	proxy := &httputil.ReverseProxy{
		Rewrite: func(req *httputil.ProxyRequest) {
			k8sproxy.Rewrite(req)

			req.Out.URL.Scheme = "http"
			req.Out.URL.Host = apiserver.Listener.Addr().String()
		},
	}

	// the middleware drops whatever the client sent and sets the identity it verified
	omni := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, req *http.Request) {
		req.Header.Del(transport.ImpersonateUserHeader)
		req.Header.Del(transport.ImpersonateGroupHeader)
		req.Header.Add(transport.ImpersonateUserHeader, "caller@example.com")
		req.Header.Add(transport.ImpersonateGroupHeader, "caller-group")

		proxy.ServeHTTP(writer, req)
	}))

	t.Cleanup(omni.Close)

	for _, test := range []struct {
		header http.Header
		name   string
	}{
		{name: "an ordinary request", header: http.Header{}},
		{name: "a request that lists headers in Connection", header: http.Header{"Connection": {"Impersonate-User, Impersonate-Group"}}},
		{
			name: "a request that carries impersonation headers",
			header: http.Header{
				"Impersonate-User": {"someone-else"}, "Impersonate-Group": {"some-group"},
				"Impersonate-Uid": {"some-uid"}, "Impersonate-Extra-Scopes": {"some-value"},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, omni.URL, nil)
			require.NoError(t, err)

			req.Header = test.header

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			require.NoError(t, resp.Body.Close())

			assert.Equal(t, []string{"caller@example.com"}, upstream.Values(transport.ImpersonateUserHeader))
			assert.Equal(t, []string{"caller-group"}, upstream.Values(transport.ImpersonateGroupHeader))
			assert.Empty(t, upstream.Values(transport.ImpersonateUIDHeader))
			assert.Empty(t, upstream.Values("Impersonate-Extra-Scopes"))
		})
	}
}
