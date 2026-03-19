// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package workloadproxy_test

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/omni/internal/backend/services/workloadproxy"
)

var redirectSignature = []byte("1234")

const testPublicKeyID = "test-public-key-id"

type mockProxyProvider struct {
	aliases []string
}

func (m *mockProxyProvider) GetProxy(alias string) (http.Handler, resource.ID, error) {
	m.aliases = append(m.aliases, alias)

	return http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		// return 200 with info on the request
		writer.WriteHeader(http.StatusOK)

		writer.Write([]byte("alias: " + alias)) //nolint:errcheck
	}), "test-cluster", nil
}

type mockAccessValidator struct {
	publicKeyIDs                []string
	publicKeyIDSignatureBase64s []string
	clusterIDs                  []resource.ID
}

func (m *mockAccessValidator) ValidateAccess(_ context.Context, publicKeyID, publicKeyIDSignatureBase64 string, clusterID resource.ID) error {
	m.publicKeyIDs = append(m.publicKeyIDs, publicKeyID)
	m.publicKeyIDSignatureBase64s = append(m.publicKeyIDSignatureBase64s, publicKeyIDSignatureBase64)
	m.clusterIDs = append(m.clusterIDs, clusterID)

	return nil
}

type mockHandler struct {
	requests []*http.Request
}

func (m *mockHandler) ServeHTTP(_ http.ResponseWriter, request *http.Request) {
	m.requests = append(m.requests, request)
}

func TestHandler(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	t.Cleanup(cancel)

	mainURL, err := url.Parse("https://instanceid.example.com")
	require.NoError(t, err)

	t.Run("not a subdomain request", func(t *testing.T) {
		t.Parallel()

		next := &mockHandler{}
		proxyProvider := &mockProxyProvider{}
		accessValidator := &mockAccessValidator{}
		logger := zaptest.NewLogger(t)

		handler, err := workloadproxy.NewHTTPHandler(next, proxyProvider, accessValidator, mainURL, "proxy-us", false, logger, redirectSignature)
		require.NoError(t, err)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://instanceid.example.com/example", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		require.Equal(t, http.StatusOK, rr.Code)
		require.Len(t, next.requests, 1)
	})

	t.Run("subdomain request without cookies", func(t *testing.T) {
		t.Parallel()

		next := &mockHandler{}
		proxyProvider := &mockProxyProvider{}
		accessValidator := &mockAccessValidator{}
		logger := zaptest.NewLogger(t)

		handler, err := workloadproxy.NewHTTPHandler(next, proxyProvider, accessValidator, mainURL, "proxy-us", false, logger, redirectSignature)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		testServiceAlias := "testsvc"

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("https://%s-instanceid.proxy-us.example.com/example", testServiceAlias), nil)
		require.NoError(t, err)

		handler.ServeHTTP(rr, req)

		require.Equal(t, []string{testServiceAlias}, proxyProvider.aliases)

		require.Equal(t, http.StatusSeeOther, rr.Code)

		redirectedURL, err := url.Parse(rr.Header().Get("Location"))
		require.NoError(t, err)

		redirectedURL.Port()

		redirectBackURL := redirectedURL.Query().Get("redirect")

		redirectBackURL, err = workloadproxy.DecodeRedirectURL(redirectBackURL, redirectSignature)

		require.NoError(t, err)

		expectedHost := fmt.Sprintf("%s-instanceid.proxy-us.example.com", testServiceAlias)
		if port := redirectedURL.Port(); port != "" {
			expectedHost = fmt.Sprintf("%s:%s", expectedHost, port)
		}

		require.Equal(t, fmt.Sprintf("https://%s/example", expectedHost), redirectBackURL)
	})

	t.Run("subdomain request with cookies", func(t *testing.T) {
		t.Parallel()

		testSubdomainRequestWithCookies(ctx, t, mainURL, "instanceid")
	})

	t.Run("subdomain request with cookies - dash in instance name", func(t *testing.T) {
		t.Parallel()

		testSubdomainRequestWithCookies(ctx, t, mainURL, "instance-id")
	})

	t.Run("subdomain request with cookies - legacy format", func(t *testing.T) {
		t.Parallel()

		next := &mockHandler{}
		proxyProvider := &mockProxyProvider{}
		accessValidator := &mockAccessValidator{}
		logger := zaptest.NewLogger(t)

		handler, err := workloadproxy.NewHTTPHandler(next, proxyProvider, accessValidator, mainURL, "proxy-us", false, logger, redirectSignature)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		testServiceAlias := "testsvc2"

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("https://%s-%s-instanceid.example.com/example", workloadproxy.LegacyHostPrefix, testServiceAlias), nil)
		require.NoError(t, err)

		testPublicKeyIDSignatureBase64 := base64.StdEncoding.EncodeToString([]byte("test-signed-public-key-id"))

		req.AddCookie(&http.Cookie{Name: workloadproxy.PublicKeyIDCookie, Value: testPublicKeyID})
		req.AddCookie(&http.Cookie{Name: workloadproxy.PublicKeyIDSignatureBase64Cookie, Value: testPublicKeyIDSignatureBase64})

		handler.ServeHTTP(rr, req)

		require.Equal(t, []string{testServiceAlias}, proxyProvider.aliases)

		require.Equal(t, http.StatusOK, rr.Code)

		require.Equal(t, []string{testPublicKeyID}, accessValidator.publicKeyIDs)
		require.Equal(t, []string{testPublicKeyIDSignatureBase64}, accessValidator.publicKeyIDSignatureBase64s)
		require.Equal(t, []resource.ID{"test-cluster"}, accessValidator.clusterIDs)
	})
}

func TestHandlerUseOmniSubdomain(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	t.Cleanup(cancel)

	mainURL, err := url.Parse("https://omni.example.com")
	require.NoError(t, err)

	t.Run("not a proxy request", func(t *testing.T) {
		t.Parallel()

		next := &mockHandler{}
		proxyProvider := &mockProxyProvider{}
		accessValidator := &mockAccessValidator{}
		logger := zaptest.NewLogger(t)

		handler, err := workloadproxy.NewHTTPHandler(next, proxyProvider, accessValidator, mainURL, "proxy", true, logger, redirectSignature)
		require.NoError(t, err)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://omni.example.com/example", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		require.Equal(t, http.StatusOK, rr.Code)
		require.Len(t, next.requests, 1)
		require.Empty(t, proxyProvider.aliases)
	})

	t.Run("proxy request with cookies", func(t *testing.T) {
		t.Parallel()

		testUseOmniSubdomainWithCookies(ctx, t, mainURL, "proxy", "https://my-service.proxy.omni.example.com/example", "my-service")
	})

	t.Run("proxy request with dashes in alias", func(t *testing.T) {
		t.Parallel()

		testUseOmniSubdomainWithCookies(ctx, t, mainURL, "proxy", "https://my-cool-service.proxy.omni.example.com/path", "my-cool-service")
	})

	t.Run("proxy request without cookies redirects to login", func(t *testing.T) {
		t.Parallel()

		next := &mockHandler{}
		proxyProvider := &mockProxyProvider{}
		accessValidator := &mockAccessValidator{}
		logger := zaptest.NewLogger(t)

		handler, err := workloadproxy.NewHTTPHandler(next, proxyProvider, accessValidator, mainURL, "proxy", true, logger, redirectSignature)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://grafana.proxy.omni.example.com/dashboard", nil)
		require.NoError(t, err)

		handler.ServeHTTP(rr, req)

		require.Equal(t, []string{"grafana"}, proxyProvider.aliases)
		require.Equal(t, http.StatusSeeOther, rr.Code)

		redirectedURL, err := url.Parse(rr.Header().Get("Location"))
		require.NoError(t, err)

		require.Equal(t, "omni.example.com", redirectedURL.Host)
		require.Equal(t, "/authenticate", redirectedURL.Path)
	})
}

func TestHandlerUseOmniSubdomainEmptySubdomain(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	t.Cleanup(cancel)

	mainURL, err := url.Parse("https://omni.example.com")
	require.NoError(t, err)

	t.Run("proxy request with cookies", func(t *testing.T) {
		t.Parallel()

		testUseOmniSubdomainWithCookies(ctx, t, mainURL, "", "https://grafana.omni.example.com/dashboard", "grafana")
	})

	t.Run("omni itself is not a proxy request", func(t *testing.T) {
		t.Parallel()

		next := &mockHandler{}
		proxyProvider := &mockProxyProvider{}
		accessValidator := &mockAccessValidator{}
		logger := zaptest.NewLogger(t)

		handler, err := workloadproxy.NewHTTPHandler(next, proxyProvider, accessValidator, mainURL, "", true, logger, redirectSignature)
		require.NoError(t, err)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://omni.example.com/", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		require.Equal(t, http.StatusOK, rr.Code)
		require.Len(t, next.requests, 1)
		require.Empty(t, proxyProvider.aliases)
	})
}

func testUseOmniSubdomainWithCookies(ctx context.Context, t *testing.T, mainURL *url.URL, subdomain, requestURL, expectedAlias string) {
	t.Helper()

	next := &mockHandler{}
	proxyProvider := &mockProxyProvider{}
	accessValidator := &mockAccessValidator{}
	logger := zaptest.NewLogger(t)

	handler, err := workloadproxy.NewHTTPHandler(next, proxyProvider, accessValidator, mainURL, subdomain, true, logger, redirectSignature)
	require.NoError(t, err)

	rr := httptest.NewRecorder()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	require.NoError(t, err)

	testPublicKeyIDSignatureBase64 := base64.StdEncoding.EncodeToString([]byte("test-signed-public-key-id"))

	req.AddCookie(&http.Cookie{Name: workloadproxy.PublicKeyIDCookie, Value: testPublicKeyID})
	req.AddCookie(&http.Cookie{Name: workloadproxy.PublicKeyIDSignatureBase64Cookie, Value: testPublicKeyIDSignatureBase64})

	handler.ServeHTTP(rr, req)

	require.Equal(t, []string{expectedAlias}, proxyProvider.aliases)
	require.Equal(t, http.StatusOK, rr.Code)
}

func testSubdomainRequestWithCookies(ctx context.Context, t *testing.T, mainURL *url.URL, instanceID string) {
	next := &mockHandler{}
	proxyProvider := &mockProxyProvider{}
	accessValidator := &mockAccessValidator{}
	logger := zaptest.NewLogger(t)

	handler, err := workloadproxy.NewHTTPHandler(next, proxyProvider, accessValidator, mainURL, "proxy-us", false, logger, redirectSignature)
	require.NoError(t, err)

	rr := httptest.NewRecorder()

	testServiceAlias := "testsvc2"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("https://%s-%s.proxy-us.example.com/example", testServiceAlias, instanceID), nil)
	require.NoError(t, err)

	testPublicKeyIDSignatureBase64 := base64.StdEncoding.EncodeToString([]byte("test-signed-public-key-id"))

	req.AddCookie(&http.Cookie{Name: workloadproxy.PublicKeyIDCookie, Value: testPublicKeyID})
	req.AddCookie(&http.Cookie{Name: workloadproxy.PublicKeyIDSignatureBase64Cookie, Value: testPublicKeyIDSignatureBase64})

	handler.ServeHTTP(rr, req)

	require.Equal(t, []string{testServiceAlias}, proxyProvider.aliases)

	require.Equal(t, http.StatusOK, rr.Code)

	require.Equal(t, []string{testPublicKeyID}, accessValidator.publicKeyIDs)
	require.Equal(t, []string{testPublicKeyIDSignatureBase64}, accessValidator.publicKeyIDSignatureBase64s)
	require.Equal(t, []resource.ID{"test-cluster"}, accessValidator.clusterIDs)
}
