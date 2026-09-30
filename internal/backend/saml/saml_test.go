// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package saml_test

import (
	"bytes"
	"compress/flate"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"html"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/beevik/etree"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/crewjam/saml"
	"github.com/crewjam/saml/samlsp"
	dsig "github.com/russellhaering/goxmldsig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"go.uber.org/zap/zaptest/observer"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	omnisaml "github.com/siderolabs/omni/internal/backend/saml"
)

const (
	testAdvertisedURL = "https://omni.example.com"
	testIDPSLOURL     = "https://idp.example.com/saml"
	testNameID        = "user@example.com"
)

// makeSLOCookie builds a saml_name_id cookie value the same way CreateSession does.
func makeSLOCookie(t *testing.T, format, sessionIndex string) *http.Cookie {
	t.Helper()

	return makeSLOCookieIssuedAt(t, format, sessionIndex, time.Now())
}

func makeSLOCookieIssuedAt(t *testing.T, format, sessionIndex string, issuedAt time.Time) *http.Cookie {
	t.Helper()

	data := map[string]any{"n": testNameID, "t": issuedAt.Unix()}

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

// newMiddleware builds an unsigned middleware whose IdP advertises an SLO endpoint for each of the given bindings.
func newMiddleware(t *testing.T, sloBindings ...string) *samlsp.Middleware {
	t.Helper()

	rootURL, err := url.Parse("https://omni.example.com")
	require.NoError(t, err)

	metadataURL := rootURL.ResolveReference(&url.URL{Path: "saml/metadata"})
	acsURL := rootURL.ResolveReference(&url.URL{Path: "saml/acs"})
	sloURL := rootURL.ResolveReference(&url.URL{Path: "saml/slo"})

	m := &samlsp.Middleware{
		ServiceProvider: saml.ServiceProvider{
			MetadataURL: *metadataURL,
			AcsURL:      *acsURL,
			SloURL:      *sloURL,
			IDPMetadata: &saml.EntityDescriptor{
				EntityID: "https://idp.example.com",
			},
		},
	}

	setSLOBindings(m, sloBindings...)

	return m
}

func setSLOBindings(m *samlsp.Middleware, sloBindings ...string) {
	endpoints := make([]saml.Endpoint, 0, len(sloBindings))

	for _, binding := range sloBindings {
		endpoints = append(endpoints, saml.Endpoint{Binding: binding, Location: testIDPSLOURL})
	}

	m.ServiceProvider.IDPMetadata.IDPSSODescriptors = []saml.IDPSSODescriptor{{SingleLogoutServices: endpoints}}
}

// newHandler builds a handler the way the server does, backed by a state that has SAML enabled.
func newHandler(t *testing.T, signingCert *tls.Certificate) *samlsp.Middleware {
	t.Helper()

	st := state.WrapCore(namespaced.NewState(inmem.Build))

	authConfig := auth.NewAuthConfig()
	authConfig.TypedSpec().Value.Saml = &specs.AuthConfigSpec_SAML{}

	require.NoError(t, st.Create(t.Context(), authConfig))

	m, err := omnisaml.NewHandler(
		st,
		&specs.AuthConfigSpec_SAML{Metadata: "testdata/samlsp_metadata.xml"},
		zaptest.NewLogger(t),
		testAdvertisedURL,
		"",
		true,
		signingCert,
	)
	require.NoError(t, err)

	return m
}

func newLogoutHandler(t *testing.T, m *samlsp.Middleware) http.HandlerFunc {
	t.Helper()

	handler, err := omnisaml.CreateLogoutHandler(m, testAdvertisedURL, zaptest.NewLogger(t))
	require.NoError(t, err)

	return handler
}

func newSigningCert(t *testing.T, key crypto.Signer) *tls.Certificate {
	t.Helper()

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "omni.example.com"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, key.Public(), key)
	require.NoError(t, err)

	leaf, err := x509.ParseCertificate(der)
	require.NoError(t, err)

	return &tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key, Leaf: leaf}
}

func newRSASigningCert(t *testing.T) *tls.Certificate {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	return newSigningCert(t, key)
}

// loginCookie runs an assertion through the session provider of m and returns the SLO cookie it sets.
func loginCookie(t *testing.T, m *samlsp.Middleware, sessionIndex string) *http.Cookie {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/saml/acs", nil)
	req.Form = url.Values{}

	rec := httptest.NewRecorder()

	require.NoError(t, m.Session.CreateSession(rec, req, &saml.Assertion{
		Subject:         &saml.Subject{NameID: &saml.NameID{Value: testNameID, Format: string(saml.EmailAddressNameIDFormat)}},
		AuthnStatements: []saml.AuthnStatement{{SessionIndex: sessionIndex}},
		AttributeStatements: []saml.AttributeStatement{
			{
				Attributes: []saml.Attribute{
					{Name: "email", Values: []saml.AttributeValue{{Value: testNameID}}},
				},
			},
		},
	}))

	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == omnisaml.NameIDCookieName {
			return cookie
		}
	}

	require.FailNow(t, "expected the login to set the SLO cookie")

	return nil
}

func serveLogout(t *testing.T, handler http.HandlerFunc, cookie *http.Cookie) *http.Response {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/logout", nil)

	if cookie != nil {
		req.AddCookie(cookie)
	}

	rec := httptest.NewRecorder()

	handler(rec, req)

	return rec.Result()
}

var samlRequestInput = regexp.MustCompile(`name="SAMLRequest" value="([^"]*)"`)

// postedLogoutRequest returns the LogoutRequest XML from the auto-submitting form the handler responds with.
func postedLogoutRequest(t *testing.T, resp *http.Response) []byte {
	t.Helper()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "text/html", resp.Header.Get("Content-Type"))

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	match := samlRequestInput.FindSubmatch(body)
	require.NotNil(t, match, "expected a SAMLRequest form input in %q", body)

	logoutRequest, err := base64.StdEncoding.DecodeString(html.UnescapeString(string(match[1])))
	require.NoError(t, err)

	return logoutRequest
}

// startAuthFlow builds a handler the way the server does and runs one login through it, returning the
// response so both the AuthnRequest and the cookies it set can be inspected.
func startAuthFlow(t *testing.T, signingCert *tls.Certificate) *http.Response {
	t.Helper()

	m := newHandler(t, signingCert)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/login?flow=frontend", nil)
	rec := httptest.NewRecorder()

	m.HandleStartAuthFlow(rec, req)

	return rec.Result()
}

// TestLoginForcesReauthentication pins the behavior that makes logging out of Omni mean something. The
// IdP session survives an Omni logout whenever single logout is unavailable, so without ForceAuthn the
// next AuthnRequest is answered with the user who just logged out.
func TestLoginForcesReauthentication(t *testing.T) {
	resp := startAuthFlow(t, nil)
	defer resp.Body.Close() //nolint:errcheck

	require.Equal(t, http.StatusFound, resp.StatusCode)

	redirectURL, err := url.Parse(resp.Header.Get("Location"))
	require.NoError(t, err)

	deflated, err := base64.StdEncoding.DecodeString(redirectURL.Query().Get("SAMLRequest"))
	require.NoError(t, err)

	r := flate.NewReader(bytes.NewReader(deflated))
	defer r.Close() //nolint:errcheck

	authnRequest, err := io.ReadAll(r)
	require.NoError(t, err)

	assert.Contains(t, string(authnRequest), `ForceAuthn="true"`)
}

// TestTrackedRequestOutlastsAPerson guards the cookie that correlates the AuthnRequest with the response.
// The library defaults it to MaxIssueDelay, 90 seconds, which was ample while this was a silent redirect.
// Now that every login means typing a password and clearing MFA, a short lifetime sends anyone who takes
// their time to /forbidden instead of into Omni.
func TestTrackedRequestOutlastsAPerson(t *testing.T) {
	resp := startAuthFlow(t, nil)
	defer resp.Body.Close() //nolint:errcheck

	for _, cookie := range resp.Cookies() {
		if !strings.HasPrefix(cookie.Name, "saml_") {
			continue
		}

		assert.Greater(t, cookie.MaxAge, int(saml.MaxIssueDelay.Seconds()),
			"the tracked request cookie %q expires after %ds, too soon for a password and an MFA challenge",
			cookie.Name, cookie.MaxAge)

		return
	}

	t.Error("expected the auth flow to set a tracked request cookie")
}

func TestCreateLogoutHandler_NoCookie(t *testing.T) {
	handler := newLogoutHandler(t, newMiddleware(t, saml.HTTPRedirectBinding))

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
	handler := newLogoutHandler(t, newMiddleware(t))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/logout", nil)
	req.AddCookie(makeSLOCookie(t, "", ""))

	rec := httptest.NewRecorder()

	handler(rec, req)

	resp := rec.Result()
	defer resp.Body.Close() //nolint:errcheck

	assert.Equal(t, http.StatusSeeOther, resp.StatusCode)
	assert.Equal(t, testAdvertisedURL, resp.Header.Get("Location"))

	assertNameIDCookieCleared(t, resp)
}

func TestCreateLogoutHandler_RedirectsToSLO(t *testing.T) {
	handler := newLogoutHandler(t, newMiddleware(t, saml.HTTPRedirectBinding))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/logout", nil)
	req.AddCookie(makeSLOCookie(t, "urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress", "_session123"))

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

	// The cookie has to survive until the IdP answers, otherwise a speculative GET
	// that never follows the redirect would leave the real navigation unable to
	// build a LogoutRequest.
	assertNameIDCookieKept(t, resp)
}

func TestCreateLogoutHandler_RepeatedRequestStillRedirectsToSLO(t *testing.T) {
	handler := newLogoutHandler(t, newMiddleware(t, saml.HTTPRedirectBinding))
	cookie := makeSLOCookie(t, "urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress", "_session123")

	for range 2 {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/logout", nil)
		req.AddCookie(cookie)

		rec := httptest.NewRecorder()

		handler(rec, req)

		resp := rec.Result()
		location := resp.Header.Get("Location")
		_ = resp.Body.Close() //nolint:errcheck

		assert.Equal(t, http.StatusFound, resp.StatusCode)

		redirectURL, err := url.Parse(location)
		require.NoError(t, err)

		assert.Equal(t, "idp.example.com", redirectURL.Host)
		assert.NotEmpty(t, redirectURL.Query().Get("SAMLRequest"))
	}
}

func TestCreateLogoutHandler_InvalidCookieValue(t *testing.T) {
	handler := newLogoutHandler(t, newMiddleware(t, saml.HTTPRedirectBinding))

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

func TestCreateLogoutHandler_ExpiredCookie(t *testing.T) {
	handler := newLogoutHandler(t, newMiddleware(t, saml.HTTPRedirectBinding))

	resp := serveLogout(t, handler, makeSLOCookieIssuedAt(t, "", "_session123", time.Now().Add(-48*time.Hour)))
	defer resp.Body.Close() //nolint:errcheck

	assert.Equal(t, http.StatusSeeOther, resp.StatusCode)
	assert.Equal(t, testAdvertisedURL, resp.Header.Get("Location"))

	assertNameIDCookieCleared(t, resp)
}

func TestSLOHandler_InvalidResponse_ClearsCookieAndRedirects(t *testing.T) {
	m := newMiddleware(t, saml.HTTPRedirectBinding)
	mux := http.NewServeMux()
	core, logs := observer.New(zap.WarnLevel)

	omnisaml.RegisterHandlers(m, mux, zap.New(core), testAdvertisedURL)

	// Provide a minimal (invalid) SAMLResponse so ValidateLogoutResponseRequest
	// does not panic on a nil XML document. The handler logs the validation error
	// and proceeds to clear the cookie and redirect.
	samlResponse := base64.StdEncoding.EncodeToString([]byte(`<samlp:LogoutResponse xmlns:samlp="urn:oasis:names:tc:SAML:2.0:protocol" ID="_fake" Version="2.0"/>`))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/saml/slo", strings.NewReader("SAMLResponse="+url.QueryEscape(samlResponse)))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(makeSLOCookie(t, "", ""))

	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	resp := rec.Result()
	defer resp.Body.Close() //nolint:errcheck

	assert.Equal(t, http.StatusSeeOther, resp.StatusCode)
	assert.Equal(t, testAdvertisedURL, resp.Header.Get("Location"))

	assertNameIDCookieCleared(t, resp)

	entries := logs.FilterMessage("invalid SAML logout response").All()
	require.Len(t, entries, 1)
	assert.Contains(t, entries[0].ContextMap()["error"], "signature", "the log has to carry the private error, not the generic public one")
}

func TestSLOHandler_NoResponse(t *testing.T) {
	m := newMiddleware(t, saml.HTTPRedirectBinding)
	mux := http.NewServeMux()
	core, logs := observer.New(zap.WarnLevel)

	omnisaml.RegisterHandlers(m, mux, zap.New(core), testAdvertisedURL)

	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/saml/slo", nil))

	resp := rec.Result()
	defer resp.Body.Close() //nolint:errcheck

	assert.Equal(t, http.StatusSeeOther, resp.StatusCode)
	assert.Equal(t, testAdvertisedURL, resp.Header.Get("Location"))

	entries := logs.FilterMessage("invalid SAML logout response").All()
	require.Len(t, entries, 1)
	assert.Equal(t, "logout response contains no XML element", entries[0].ContextMap()["error"])
}

func TestSLOHandler_IdPInitiatedLogoutRequest(t *testing.T) {
	for _, tt := range []struct {
		name         string
		fetchDest    string
		expectedCode int
	}{
		{name: "hidden iframe", fetchDest: "iframe", expectedCode: http.StatusOK},
		{name: "top-level redirect", fetchDest: "document", expectedCode: http.StatusSeeOther},
		{name: "no fetch metadata", expectedCode: http.StatusSeeOther},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := newMiddleware(t, saml.HTTPRedirectBinding)
			mux := http.NewServeMux()
			core, logs := observer.New(zap.WarnLevel)

			omnisaml.RegisterHandlers(m, mux, zap.New(core), testAdvertisedURL)

			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/saml/slo?SAMLRequest=abc", nil)
			req.AddCookie(makeSLOCookie(t, "", ""))

			if tt.fetchDest != "" {
				req.Header.Set("Sec-Fetch-Dest", tt.fetchDest)
			}

			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			resp := rec.Result()
			defer resp.Body.Close() //nolint:errcheck

			assert.Equal(t, tt.expectedCode, resp.StatusCode)

			if tt.expectedCode == http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				require.NoError(t, err)

				assert.Empty(t, body)
				assert.Empty(t, resp.Header.Get("Location"))
			} else {
				assert.Equal(t, testAdvertisedURL, resp.Header.Get("Location"))

				assertNameIDCookieCleared(t, resp)
			}

			assert.Len(t, logs.FilterMessage("IdP-initiated SAML logout is not supported, ignoring the logout request").All(), 1)
			assert.Empty(t, logs.FilterMessage("invalid SAML logout response").All())
		})
	}
}

func TestSLOHandler_NoCookie(t *testing.T) {
	m := newMiddleware(t, saml.HTTPRedirectBinding)
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

func TestACS_LogoutResponseOverRedirectBinding(t *testing.T) {
	m := newMiddleware(t, saml.HTTPRedirectBinding)
	mux := http.NewServeMux()

	omnisaml.RegisterHandlers(m, mux, zaptest.NewLogger(t), testAdvertisedURL)

	// An IdP without forced POST binding answers the LogoutRequest by redirecting the
	// browser back to the ACS URL with the response in the query string.
	samlResponse := base64.StdEncoding.EncodeToString([]byte(`<samlp:LogoutResponse xmlns:samlp="urn:oasis:names:tc:SAML:2.0:protocol" ID="_fake" Version="2.0"/>`))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet,
		"/saml/acs?SAMLResponse="+url.QueryEscape(samlResponse)+"&RelayState="+url.QueryEscape(testAdvertisedURL), nil)
	req.AddCookie(makeSLOCookie(t, "", ""))

	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	resp := rec.Result()
	defer resp.Body.Close() //nolint:errcheck

	assert.Equal(t, http.StatusSeeOther, resp.StatusCode)
	assert.Equal(t, testAdvertisedURL, resp.Header.Get("Location"))

	assertNameIDCookieCleared(t, resp)
}

// RegisterHandlers gives the ACS path its own mux entry as a literal, so that literal
// has to keep matching the path the middleware resolves for itself. A mismatch is silent:
// the request falls through to the middleware, which does not recognize the path either.
func TestACSPathMatchesServiceProvider(t *testing.T) {
	assert.Equal(t, "/saml/acs", newMiddleware(t, saml.HTTPRedirectBinding).ServiceProvider.AcsURL.Path)
}

func assertNameIDCookieKept(t *testing.T, resp *http.Response) {
	t.Helper()

	for _, cookie := range resp.Cookies() {
		if cookie.Name == omnisaml.NameIDCookieName {
			t.Errorf("expected saml_name_id cookie to be left untouched, got value %q with MaxAge %d", cookie.Value, cookie.MaxAge)

			return
		}
	}
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

func TestAllowIDPInitiated(t *testing.T) {
	t.Parallel()

	for _, allow := range []bool{true, false} {
		m, err := omnisaml.NewHandler(
			state.WrapCore(namespaced.NewState(inmem.Build)),
			&specs.AuthConfigSpec_SAML{Metadata: "testdata/samlsp_metadata.xml"},
			zaptest.NewLogger(t),
			testAdvertisedURL,
			"",
			allow,
			nil,
		)
		require.NoError(t, err)

		assert.Equal(t, allow, m.ServiceProvider.AllowIDPInitiated)
	}
}

func TestLoginSignsAuthnRequest(t *testing.T) {
	cert := newRSASigningCert(t)

	resp := startAuthFlow(t, cert)
	defer resp.Body.Close() //nolint:errcheck

	require.Equal(t, http.StatusFound, resp.StatusCode)

	redirectURL, err := url.Parse(resp.Header.Get("Location"))
	require.NoError(t, err)

	assertRedirectSignature(t, cert, redirectURL)
}

func TestMetadataAdvertisesSigningCert(t *testing.T) {
	cert := newRSASigningCert(t)

	descriptor := fetchMetadata(t, newHandler(t, cert))

	require.NotNil(t, descriptor.AuthnRequestsSigned)
	assert.True(t, *descriptor.AuthnRequestsSigned)

	var signingCerts []string

	for _, keyDescriptor := range descriptor.KeyDescriptors {
		if keyDescriptor.Use != "signing" {
			continue
		}

		for _, x509Cert := range keyDescriptor.KeyInfo.X509Data.X509Certificates {
			signingCerts = append(signingCerts, x509Cert.Data)
		}
	}

	require.Len(t, signingCerts, 1)

	der, err := base64.StdEncoding.DecodeString(signingCerts[0])
	require.NoError(t, err)

	assert.Equal(t, cert.Leaf.Raw, der)
}

func TestMetadataWithoutSigningCertHasNoKeys(t *testing.T) {
	descriptor := fetchMetadata(t, newHandler(t, nil))

	require.NotNil(t, descriptor.AuthnRequestsSigned)
	assert.False(t, *descriptor.AuthnRequestsSigned)
	assert.Empty(t, descriptor.KeyDescriptors)
}

func fetchMetadata(t *testing.T, m *samlsp.Middleware) saml.SPSSODescriptor {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/saml/metadata", nil)
	rec := httptest.NewRecorder()

	m.ServeMetadata(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var entity saml.EntityDescriptor

	require.NoError(t, xml.Unmarshal(rec.Body.Bytes(), &entity))
	require.Len(t, entity.SPSSODescriptors, 1)

	return entity.SPSSODescriptors[0]
}

func TestNewHandlerRejectsNonRSASigningKey(t *testing.T) {
	ecdsaKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	_, ed25519Key, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	for _, tt := range []struct {
		key  crypto.Signer
		name string
	}{
		{name: "ecdsa", key: ecdsaKey},
		{name: "ed25519", key: ed25519Key},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := omnisaml.NewHandler(
				state.WrapCore(namespaced.NewState(inmem.Build)),
				&specs.AuthConfigSpec_SAML{Metadata: "testdata/samlsp_metadata.xml"},
				zaptest.NewLogger(t),
				testAdvertisedURL,
				"",
				true,
				newSigningCert(t, tt.key),
			)
			require.ErrorContains(t, err, "only RSA keys are supported")
		})
	}
}

func TestCreateLogoutHandler_SignedPostsSignedRequest(t *testing.T) {
	cert := newRSASigningCert(t)
	m := newHandler(t, cert)

	setSLOBindings(m, saml.HTTPPostBinding)

	resp := serveLogout(t, newLogoutHandler(t, m), loginCookie(t, m, "_session123"))
	defer resp.Body.Close() //nolint:errcheck

	doc := etree.NewDocument()
	require.NoError(t, doc.ReadFromBytes(postedLogoutRequest(t, resp)))

	validationCtx := dsig.NewDefaultValidationContext(&dsig.MemoryX509CertificateStore{Roots: []*x509.Certificate{cert.Leaf}})

	signed, err := validationCtx.Validate(doc.Root())
	require.NoError(t, err)

	var logoutRequest saml.LogoutRequest

	signedDoc := etree.NewDocument()
	signedDoc.SetRoot(signed)

	signedXML, err := signedDoc.WriteToBytes()
	require.NoError(t, err)

	require.NoError(t, xml.Unmarshal(signedXML, &logoutRequest))

	assert.Equal(t, testIDPSLOURL, logoutRequest.Destination)
	require.NotNil(t, logoutRequest.NameID)
	assert.Equal(t, testNameID, logoutRequest.NameID.Value)
	assert.Equal(t, string(saml.EmailAddressNameIDFormat), logoutRequest.NameID.Format)
	require.NotNil(t, logoutRequest.SessionIndex, "the session index from the cookie has to be covered by the signature")
	assert.Equal(t, "_session123", logoutRequest.SessionIndex.Value)

	assertNameIDCookieKept(t, resp)
}

func TestCreateLogoutHandler_SignedRedirectSignsQuery(t *testing.T) {
	for _, tt := range []struct {
		name     string
		bindings []string
	}{
		{name: "redirect only, as Entra ID advertises", bindings: []string{saml.HTTPRedirectBinding}},
		{name: "post and redirect", bindings: []string{saml.HTTPPostBinding, saml.HTTPRedirectBinding}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cert := newRSASigningCert(t)
			m := newHandler(t, cert)

			setSLOBindings(m, tt.bindings...)

			resp := serveLogout(t, newLogoutHandler(t, m), loginCookie(t, m, "_session123"))
			defer resp.Body.Close() //nolint:errcheck

			require.Equal(t, http.StatusFound, resp.StatusCode)

			redirectURL, err := url.Parse(resp.Header.Get("Location"))
			require.NoError(t, err)

			assert.Equal(t, "idp.example.com", redirectURL.Host)
			assert.Equal(t, testAdvertisedURL, redirectURL.Query().Get("RelayState"))

			assertRedirectSignature(t, cert, redirectURL)

			deflated, err := base64.StdEncoding.DecodeString(redirectURL.Query().Get("SAMLRequest"))
			require.NoError(t, err)

			r := flate.NewReader(bytes.NewReader(deflated))
			defer r.Close() //nolint:errcheck

			logoutRequest, err := io.ReadAll(r)
			require.NoError(t, err)

			assert.Contains(t, string(logoutRequest), "_session123")
			assert.NotContains(t, string(logoutRequest), "Signature", "the redirect binding carries the signature in the query string only")

			assertNameIDCookieKept(t, resp)
		})
	}
}

// assertRedirectSignature checks the query string signature the way an IdP does: over the raw, still
// URL-encoded SAMLRequest, RelayState (when present) and SigAlg parameters, in that order.
func assertRedirectSignature(t *testing.T, cert *tls.Certificate, redirectURL *url.URL) {
	t.Helper()

	rawParams := map[string]string{}

	for param := range strings.SplitSeq(redirectURL.RawQuery, "&") {
		key, value, _ := strings.Cut(param, "=")
		rawParams[key] = value
	}

	query := redirectURL.Query()

	assert.Equal(t, dsig.RSASHA256SignatureMethod, query.Get("SigAlg"))

	signature, err := base64.StdEncoding.DecodeString(query.Get("Signature"))
	require.NoError(t, err)

	signedQuery := "SAMLRequest=" + rawParams["SAMLRequest"]

	if relayState, ok := rawParams["RelayState"]; ok {
		signedQuery += "&RelayState=" + relayState
	}

	signedQuery += "&SigAlg=" + rawParams["SigAlg"]

	digest := sha256.Sum256([]byte(signedQuery))

	require.NoError(t, rsa.VerifyPKCS1v15(cert.Leaf.PublicKey.(*rsa.PublicKey), crypto.SHA256, digest[:], signature)) //nolint:forcetypeassert,errcheck
}

func TestCreateLogoutHandler_SignedRejectsForgedCookie(t *testing.T) {
	m := newHandler(t, newRSASigningCert(t))

	setSLOBindings(m, saml.HTTPPostBinding)

	value, mac, found := strings.Cut(loginCookie(t, m, "_session123").Value, ".")
	require.True(t, found, "expected the SLO cookie to carry a MAC")

	raw, err := base64.URLEncoding.DecodeString(value)
	require.NoError(t, err)

	forgedData := strings.Replace(string(raw), testNameID, "admin@example.com", 1)
	forgedValue := base64.URLEncoding.EncodeToString([]byte(forgedData))

	for _, tt := range []struct {
		cookie *http.Cookie
		name   string
	}{
		{name: "changed name id", cookie: &http.Cookie{Name: omnisaml.NameIDCookieName, Value: forgedValue + "." + mac}},
		{name: "no mac", cookie: makeSLOCookie(t, "", "")},
		{name: "cookie from another key", cookie: loginCookie(t, newHandler(t, newRSASigningCert(t)), "_session123")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			resp := serveLogout(t, newLogoutHandler(t, m), tt.cookie)
			defer resp.Body.Close() //nolint:errcheck

			assert.Equal(t, http.StatusSeeOther, resp.StatusCode)
			assert.Equal(t, testAdvertisedURL, resp.Header.Get("Location"))

			assertNameIDCookieCleared(t, resp)
		})
	}
}

func TestCreateLogoutHandler_UnsignedPrefersRedirect(t *testing.T) {
	handler := newLogoutHandler(t, newMiddleware(t, saml.HTTPPostBinding, saml.HTTPRedirectBinding))

	resp := serveLogout(t, handler, makeSLOCookie(t, "", "_session123"))
	defer resp.Body.Close() //nolint:errcheck

	assert.Equal(t, http.StatusFound, resp.StatusCode)

	redirectURL, err := url.Parse(resp.Header.Get("Location"))
	require.NoError(t, err)

	assert.Equal(t, "idp.example.com", redirectURL.Host)
	assert.NotEmpty(t, redirectURL.Query().Get("SAMLRequest"))
	assert.Empty(t, redirectURL.Query().Get("Signature"))
}

func TestCreateLogoutHandler_UnsignedFallsBackToPost(t *testing.T) {
	handler := newLogoutHandler(t, newMiddleware(t, saml.HTTPPostBinding))

	resp := serveLogout(t, handler, makeSLOCookie(t, "", "_session123"))
	defer resp.Body.Close() //nolint:errcheck

	logoutRequest := string(postedLogoutRequest(t, resp))

	assert.Contains(t, logoutRequest, testNameID)
	assert.Contains(t, logoutRequest, "_session123")
	assert.NotContains(t, logoutRequest, "Signature")
}
