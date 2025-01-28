// Copyright (c) 2025 Sidero Labs, Inc.
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

	"github.com/siderolabs/omni/internal/backend/workloadproxy"
)

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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	mainURL, err := url.Parse("https://instanceid.example.com")
	require.NoError(t, err)

	t.Run("not a subdomain request", func(t *testing.T) {
		t.Parallel()

		next := &mockHandler{}
		proxyProvider := &mockProxyProvider{}
		accessValidator := &mockAccessValidator{}
		logger := zaptest.NewLogger(t)

		handler, err := workloadproxy.NewHTTPHandler(next, proxyProvider, accessValidator, mainURL, "proxy-us", logger)
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

		handler, err := workloadproxy.NewHTTPHandler(next, proxyProvider, accessValidator, mainURL, "proxy-us", logger)
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

		require.Equal(t, fmt.Sprintf("https://%s-instanceid.proxy-us.example.com:%s/example", testServiceAlias, redirectedURL.Port()), redirectBackURL)
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

		handler, err := workloadproxy.NewHTTPHandler(next, proxyProvider, accessValidator, mainURL, "proxy-us", logger)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		testServiceAlias := "testsvc2"

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("https://%s-%s-instanceid.example.com/example", workloadproxy.LegacyHostPrefix, testServiceAlias), nil)
		require.NoError(t, err)

		testPublicKeyID := "test-public-key-id"
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

func testSubdomainRequestWithCookies(ctx context.Context, t *testing.T, mainURL *url.URL, instanceID string) {
	next := &mockHandler{}
	proxyProvider := &mockProxyProvider{}
	accessValidator := &mockAccessValidator{}
	logger := zaptest.NewLogger(t)

	handler, err := workloadproxy.NewHTTPHandler(next, proxyProvider, accessValidator, mainURL, "proxy-us", logger)
	require.NoError(t, err)

	rr := httptest.NewRecorder()

	testServiceAlias := "testsvc2"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("https://%s-%s.proxy-us.example.com/example", testServiceAlias, instanceID), nil)
	require.NoError(t, err)

	testPublicKeyID := "test-public-key-id"
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
