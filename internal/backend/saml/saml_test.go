// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package saml_test

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/crewjam/saml"
	"github.com/crewjam/saml/samlsp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	omnisaml "github.com/siderolabs/omni/internal/backend/saml"
)

const (
	testAdvertisedURL = "https://omni.example.com"
	testIDPSLOURL     = "https://idp.example.com/saml"
)

// makeSLOCookie builds a saml_name_id cookie value the same way CreateSession does.
func makeSLOCookie(t *testing.T, nameID, format, sessionIndex string) *http.Cookie {
	t.Helper()

	data := map[string]string{"n": nameID}

	if format != "" {
		data["f"] = format
	}

	if sessionIndex != "" {
		data["s"] = sessionIndex
	}

	raw, err := json.Marshal(data)
	require.NoError(t, err)

	return &http.Cookie{
		Name:  omnisaml.NameIDCookieName,
		Value: base64.URLEncoding.EncodeToString(raw),
	}
}

func newMiddleware(t *testing.T, sloEndpoint string) *samlsp.Middleware {
	t.Helper()

	rootURL, err := url.Parse("https://omni.example.com")
	require.NoError(t, err)

	metadataURL := rootURL.JoinPath("saml", "metadata")
	acsURL := rootURL.JoinPath("saml", "acs")
	sloURL := rootURL.JoinPath("saml", "slo")

	sp := saml.ServiceProvider{
		MetadataURL: *metadataURL,
		AcsURL:      *acsURL,
		SloURL:      *sloURL,
		IDPMetadata: &saml.EntityDescriptor{
			EntityID: "https://idp.example.com",
		},
	}

	if sloEndpoint != "" {
		sp.IDPMetadata.IDPSSODescriptors = []saml.IDPSSODescriptor{
			{
				SSODescriptor: saml.SSODescriptor{
					SingleLogoutServices: []saml.Endpoint{
						{
							Binding:  saml.HTTPRedirectBinding,
							Location: sloEndpoint,
						},
					},
				},
			},
		}
	}

	return &samlsp.Middleware{ServiceProvider: sp}
}

func TestCreateLogoutHandler_NoCookie(t *testing.T) {
	handler := omnisaml.CreateLogoutHandler(newMiddleware(t, testIDPSLOURL), testAdvertisedURL, zaptest.NewLogger(t))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/logout", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	resp := rec.Result()
	defer resp.Body.Close() //nolint:errcheck

	assert.Equal(t, http.StatusSeeOther, resp.StatusCode)
	assert.Equal(t, testAdvertisedURL, resp.Header.Get("Location"))

	assertNameIDCookieCleared(t, resp)
}

func TestCreateLogoutHandler_NoSLOEndpoint(t *testing.T) {
	handler := omnisaml.CreateLogoutHandler(newMiddleware(t, ""), testAdvertisedURL, zaptest.NewLogger(t))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/logout", nil)
	req.AddCookie(makeSLOCookie(t, "user@example.com", "", ""))

	rec := httptest.NewRecorder()

	handler(rec, req)

	resp := rec.Result()
	defer resp.Body.Close() //nolint:errcheck

	assert.Equal(t, http.StatusSeeOther, resp.StatusCode)
	assert.Equal(t, testAdvertisedURL, resp.Header.Get("Location"))

	assertNameIDCookieCleared(t, resp)
}

func TestCreateLogoutHandler_RedirectsToSLO(t *testing.T) {
	handler := omnisaml.CreateLogoutHandler(newMiddleware(t, testIDPSLOURL), testAdvertisedURL, zaptest.NewLogger(t))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/logout", nil)
	req.AddCookie(makeSLOCookie(t, "user@example.com", "urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress", "_session123"))

	rec := httptest.NewRecorder()

	handler(rec, req)

	resp := rec.Result()
	defer resp.Body.Close() //nolint:errcheck

	assert.Equal(t, http.StatusFound, resp.StatusCode)

	location := resp.Header.Get("Location")
	redirectURL, err := url.Parse(location)
	require.NoError(t, err)

	assert.Equal(t, "idp.example.com", redirectURL.Host)
	assert.Equal(t, "/saml", redirectURL.Path)
	assert.NotEmpty(t, redirectURL.Query().Get("SAMLRequest"))
	assert.Equal(t, testAdvertisedURL, redirectURL.Query().Get("RelayState"))

	assertNameIDCookieCleared(t, resp)
}

func TestCreateLogoutHandler_InvalidCookieValue(t *testing.T) {
	handler := omnisaml.CreateLogoutHandler(newMiddleware(t, testIDPSLOURL), testAdvertisedURL, zaptest.NewLogger(t))

	for _, tt := range []struct {
		name        string
		cookieValue string
	}{
		{name: "empty value", cookieValue: ""},
		{name: "invalid base64", cookieValue: "%%%not-base64%%%"},
		{name: "invalid json", cookieValue: base64.URLEncoding.EncodeToString([]byte("{broken"))},
		{name: "missing name_id", cookieValue: base64.URLEncoding.EncodeToString([]byte(`{"f":"fmt"}`))},
	} {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/logout", nil)
			req.AddCookie(&http.Cookie{Name: omnisaml.NameIDCookieName, Value: tt.cookieValue})

			rec := httptest.NewRecorder()

			handler(rec, req)

			resp := rec.Result()
			defer resp.Body.Close() //nolint:errcheck

			assert.Equal(t, http.StatusSeeOther, resp.StatusCode)
			assert.Equal(t, testAdvertisedURL, resp.Header.Get("Location"))

			assertNameIDCookieCleared(t, resp)
		})
	}
}

func TestSLOHandler_InvalidResponse_ClearsCookieAndRedirects(t *testing.T) {
	m := newMiddleware(t, testIDPSLOURL)
	mux := http.NewServeMux()

	omnisaml.RegisterHandlers(m, mux, zaptest.NewLogger(t), testAdvertisedURL)

	// Provide a minimal (invalid) SAMLResponse so ValidateLogoutResponseRequest
	// does not panic on a nil XML document. The handler logs the validation error
	// and proceeds to clear the cookie and redirect.
	samlResponse := base64.StdEncoding.EncodeToString([]byte(`<samlp:LogoutResponse xmlns:samlp="urn:oasis:names:tc:SAML:2.0:protocol" ID="_fake" Version="2.0"/>`))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/saml/slo", strings.NewReader("SAMLResponse="+url.QueryEscape(samlResponse)))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(makeSLOCookie(t, "user@example.com", "", ""))

	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	resp := rec.Result()
	defer resp.Body.Close() //nolint:errcheck

	assert.Equal(t, http.StatusSeeOther, resp.StatusCode)
	assert.Equal(t, testAdvertisedURL, resp.Header.Get("Location"))

	assertNameIDCookieCleared(t, resp)
}

func TestSLOHandler_NoCookie(t *testing.T) {
	m := newMiddleware(t, testIDPSLOURL)
	mux := http.NewServeMux()

	omnisaml.RegisterHandlers(m, mux, zaptest.NewLogger(t), testAdvertisedURL)

	samlResponse := base64.StdEncoding.EncodeToString([]byte(`<samlp:LogoutResponse xmlns:samlp="urn:oasis:names:tc:SAML:2.0:protocol" ID="_fake" Version="2.0"/>`))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/saml/slo", strings.NewReader("SAMLResponse="+url.QueryEscape(samlResponse)))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	resp := rec.Result()
	defer resp.Body.Close() //nolint:errcheck

	// Should still redirect and clear the cookie even without a prior cookie.
	assert.Equal(t, http.StatusSeeOther, resp.StatusCode)
	assert.Equal(t, testAdvertisedURL, resp.Header.Get("Location"))

	assertNameIDCookieCleared(t, resp)
}

func assertNameIDCookieCleared(t *testing.T, resp *http.Response) {
	t.Helper()

	for _, cookie := range resp.Cookies() {
		if cookie.Name == omnisaml.NameIDCookieName {
			assert.Equal(t, "", cookie.Value)
			assert.Equal(t, -1, cookie.MaxAge)

			return
		}
	}

	t.Error("expected saml_name_id cookie to be set (cleared) in response")
}
