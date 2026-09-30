// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package saml contains SAML setup handlers.
package saml

import (
	"context"
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/crewjam/saml"
	"github.com/crewjam/saml/samlsp"
	"github.com/prometheus/client_golang/prometheus"
	dsig "github.com/russellhaering/goxmldsig"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/internal/backend/logging"
	"github.com/siderolabs/omni/internal/backend/monitoring"
)

// NameIDCookieName is the cookie used to store the SAML session data for SLO.
const NameIDCookieName = "saml_name_id"

// trackedRequestTTL is how long a pending AuthnRequest stays matchable, sized for a person completing a
// password and an MFA challenge rather than for a redirect.
const trackedRequestTTL = 15 * time.Minute

// sloSessionData holds the SAML assertion fields needed to build a LogoutRequest.
type sloSessionData struct {
	NameID       string `json:"n"`
	Format       string `json:"f,omitempty"`
	SessionIndex string `json:"s,omitempty"`
	IssuedAt     int64  `json:"t,omitempty"`
}

const sloCookieKeyInfo = "omni saml slo cookie"

// NewHandler creates new SAML handler.
//
// A nil signingCert leaves the AuthnRequest, the LogoutRequest and the SLO cookie unsigned.
func NewHandler(
	state state.State,
	cfg *specs.AuthConfigSpec_SAML,
	logger *zap.Logger,
	apiURL, recoveryAdmin string,
	allowIDPInitiated bool,
	signingCert *tls.Certificate,
) (*samlsp.Middleware, error) {
	idpMetadata, err := readMetadata(cfg)
	if err != nil {
		return nil, err
	}

	rootURL, err := url.Parse(apiURL)
	if err != nil {
		return nil, err
	}

	opts := samlsp.Options{
		URL:            *rootURL,
		IDPMetadata:    idpMetadata,
		LogoutBindings: []string{saml.HTTPRedirectBinding, saml.HTTPPostBinding},
		// The IdP session outlives an Omni logout whenever single logout is unavailable, so asking it to
		// re-authenticate is what stops it answering with whoever it still has. Without this, logging out
		// lands the same user straight back inside, and switching users is impossible.
		ForceAuthn:        true,
		AllowIDPInitiated: allowIDPInitiated,
	}

	if signingCert != nil {
		if err = setSigningCert(&opts, signingCert); err != nil {
			return nil, err
		}
	}

	serviceProvider := samlsp.DefaultServiceProvider(opts)

	if signingCert != nil {
		serviceProvider.SignatureMethod = dsig.RSASHA256SignatureMethod // override the library default (RSA-SHA1)
	}

	cookieKey, err := sloCookieKey(&serviceProvider)
	if err != nil {
		return nil, err
	}

	if cfg.NameIdFormat != "" {
		serviceProvider.AuthnNameIDFormat = saml.NameIDFormat(cfg.NameIdFormat)
	}

	requestTracker := samlsp.DefaultRequestTracker(opts, &serviceProvider)
	requestTracker.Codec = &Encoder{}
	// The library defaults this to MaxIssueDelay, 90 seconds, which was ample while the trip to the IdP
	// was a silent redirect. Now that ForceAuthn makes every login a password and an MFA challenge, the
	// cookie has to outlast a person: lose it and CreateSession cannot match the response to its request,
	// which ends at /forbidden.
	requestTracker.MaxAge = trackedRequestTTL

	m := &samlsp.Middleware{
		ServiceProvider: serviceProvider,
		ResponseBinding: saml.HTTPPostBinding,
		OnError:         createErrorHandler(logger, apiURL),
		Session: NewSessionProvider(
			state,
			requestTracker,
			logger.With(logging.Component("saml_session")),
			cfg.AttributeRules,
			recoveryAdmin,
			cookieKey,
		),
		RequestTracker:   requestTracker,
		AssertionHandler: samlsp.DefaultAssertionHandler(samlsp.Options{}),
	}

	return m, nil
}

func setSigningCert(opts *samlsp.Options, cert *tls.Certificate) error {
	key, ok := cert.PrivateKey.(*rsa.PrivateKey)
	if !ok {
		return fmt.Errorf("unsupported SAML signing key type %T, only RSA keys are supported", cert.PrivateKey)
	}

	leaf := cert.Leaf
	if leaf == nil {
		var err error

		if leaf, err = x509.ParseCertificate(cert.Certificate[0]); err != nil {
			return fmt.Errorf("failed to parse SAML signing certificate: %w", err)
		}
	}

	opts.Key = key
	opts.Certificate = leaf
	opts.SignRequest = true

	return nil
}

// sloCookieKey returns the key the SLO cookie is signed with, nil when the service provider has no signing key.
func sloCookieKey(sp *saml.ServiceProvider) ([]byte, error) {
	if sp.Key == nil {
		return nil, nil
	}

	der, err := x509.MarshalPKCS8PrivateKey(sp.Key)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal SAML signing key: %w", err)
	}

	return hkdf.Key(sha256.New, der, nil, sloCookieKeyInfo, sha256.Size)
}

// RegisterHandlers adds SAML handlers for ACS, metadata, and SLO.
func RegisterHandlers(m *samlsp.Middleware, mux *http.ServeMux, logger *zap.Logger, advertisedURL string) {
	logger = logger.With(zap.String("handler", "saml"))

	md := http.HandlerFunc(m.ServeMetadata)
	promLabel := prometheus.Labels{"handler": "saml"}
	sloHandler := createSLOHandler(m, advertisedURL, logger)

	mux.Handle("/saml/", monitoring.NewHandler(
		logging.NewHandler(m, logger),
		promLabel,
	))

	mux.Handle("/saml/metadata", monitoring.NewHandler(
		logging.NewHandler(md, logger),
		promLabel,
	))

	mux.Handle("/saml/slo", monitoring.NewHandler(
		logging.NewHandler(sloHandler, logger),
		promLabel,
	))

	// The middleware resolves the ACS path internally rather than through the mux, so
	// giving it an entry of its own is what allows a LogoutResponse to be split off
	// before the middleware tries to read it as a login response.
	mux.Handle("/saml/acs", monitoring.NewHandler(
		logging.NewHandler(routeLogoutResponse(m, sloHandler), logger),
		promLabel,
	))
}

// routeLogoutResponse sends a LogoutResponse that arrives over the HTTP-Redirect
// binding to the SLO handler rather than the ACS handler.
//
// A LogoutResponse reaches the ACS when the IdP has no dedicated SLO URL and
// falls back to a single SAML processing URL for every endpoint.
func routeLogoutResponse(acs http.Handler, slo http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Query().Get("SAMLResponse") != "" {
			slo(w, r)

			return
		}

		acs.ServeHTTP(w, r)
	}
}

// CreateLogoutHandler returns an HTTP handler that performs SAML Single Logout.
// It reads the NameID, format, and session index from a cookie set during login,
// builds a LogoutRequest, and sends it to the IdP's SLO endpoint. If the IdP
// has no usable SLO endpoint or no valid cookie is present, it falls back to a local-only logout.
func CreateLogoutHandler(m *samlsp.Middleware, advertisedURL string, logger *zap.Logger) (http.HandlerFunc, error) {
	logger = logger.With(logging.Component("saml_logout"))

	cookieKey, cookieErr := sloCookieKey(&m.ServiceProvider)
	if cookieErr != nil {
		return nil, cookieErr
	}

	logoutLocally := func(w http.ResponseWriter, r *http.Request) {
		deleteNameIDCookie(w)

		http.Redirect(w, r, advertisedURL, http.StatusSeeOther)
	}

	return func(w http.ResponseWriter, r *http.Request) {
		data, err := readNameIDCookie(r, cookieKey)
		if err != nil {
			if errors.Is(err, http.ErrNoCookie) {
				logger.Debug("no SAML SLO cookie, skipping SLO")
			} else {
				logger.Warn("invalid SAML SLO cookie, skipping SLO", zap.Error(err))
			}

			logoutLocally(w, r)

			return
		}

		binding, sloURL := pickSLOBinding(&m.ServiceProvider)
		if sloURL == "" {
			logger.Debug("IdP does not advertise an SLO endpoint, skipping SLO")

			logoutLocally(w, r)

			return
		}

		req, err := m.ServiceProvider.MakeLogoutRequest(sloURL, data.NameID)
		if err != nil {
			logger.Error("failed to build SAML logout request", zap.Error(err))

			logoutLocally(w, r)

			return
		}

		if data.Format != "" && req.NameID != nil {
			req.NameID.Format = data.Format
		}

		if data.SessionIndex != "" {
			req.SessionIndex = &saml.SessionIndex{Value: data.SessionIndex}
		}

		if binding == saml.HTTPPostBinding {
			if m.ServiceProvider.SignatureMethod != "" {
				req.Signature = nil // signed by MakeLogoutRequest before the cookie fields were set

				if err = m.ServiceProvider.SignLogoutRequest(req); err != nil {
					logger.Error("failed to sign SAML logout request", zap.Error(err))

					logoutLocally(w, r)

					return
				}
			}

			w.Header().Set("Content-Type", "text/html")

			if _, err = w.Write(req.Post(advertisedURL)); err != nil {
				logger.Warn("failed to write SAML logout request form", zap.Error(err))
			}

			return
		}

		redirectURL, err := req.Redirect(advertisedURL, &m.ServiceProvider)
		if err != nil {
			logger.Error("failed to build SAML logout redirect", zap.Error(err))

			logoutLocally(w, r)

			return
		}

		http.Redirect(w, r, redirectURL.String(), http.StatusFound)
	}, nil
}

func pickSLOBinding(sp *saml.ServiceProvider) (binding, location string) {
	for _, b := range []string{saml.HTTPRedirectBinding, saml.HTTPPostBinding} {
		if l := sp.GetSLOBindingLocation(b); l != "" {
			return b, l
		}
	}

	return "", ""
}

func createSLOHandler(m *samlsp.Middleware, advertisedURL string, logger *zap.Logger) http.HandlerFunc {
	logger = logger.With(logging.Component("saml_slo"))

	return func(w http.ResponseWriter, r *http.Request) {
		if r.FormValue("SAMLRequest") != "" {
			logger.Warn("IdP-initiated SAML logout is not supported, ignoring the logout request")

			// IdPs like Entra ID load this in a hidden iframe, where a redirect would start the frontend
			if dest := r.Header.Get("Sec-Fetch-Dest"); dest == "iframe" || dest == "frame" {
				w.WriteHeader(http.StatusOK)

				return
			}
		} else if err := m.ServiceProvider.ValidateLogoutResponseRequest(r); err != nil {
			if invalidSAML, ok := errors.AsType[*saml.InvalidResponseError](err); ok {
				err = invalidSAML.PrivateErr
			}

			logger.Warn("invalid SAML logout response", zap.Error(err))
		}

		deleteNameIDCookie(w)

		http.Redirect(w, r, advertisedURL, http.StatusSeeOther)
	}
}

// deleteNameIDCookie instructs the browser to remove the SLO cookie by setting MaxAge=-1.
func deleteNameIDCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     NameIDCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func encodeNameIDCookie(data sloSessionData, key []byte) (string, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return "", err
	}

	value := base64.URLEncoding.EncodeToString(raw)

	if key == nil {
		return value, nil
	}

	return value + "." + base64.URLEncoding.EncodeToString(nameIDCookieMAC(raw, key)), nil
}

func readNameIDCookie(r *http.Request, key []byte) (sloSessionData, error) {
	cookie, err := r.Cookie(NameIDCookieName)
	if err != nil {
		return sloSessionData{}, err
	}

	if cookie.Value == "" {
		return sloSessionData{}, http.ErrNoCookie
	}

	value := cookie.Value

	var mac []byte

	if key != nil {
		var encodedMAC string

		var found bool

		if value, encodedMAC, found = strings.Cut(value, "."); !found {
			return sloSessionData{}, errors.New("cookie is not signed")
		}

		if mac, err = base64.URLEncoding.DecodeString(encodedMAC); err != nil {
			return sloSessionData{}, fmt.Errorf("failed to decode cookie signature: %w", err)
		}
	}

	raw, err := base64.URLEncoding.DecodeString(value)
	if err != nil {
		return sloSessionData{}, fmt.Errorf("failed to decode cookie value: %w", err)
	}

	if key != nil && !hmac.Equal(mac, nameIDCookieMAC(raw, key)) {
		return sloSessionData{}, errors.New("cookie signature mismatch")
	}

	var data sloSessionData
	if err = json.Unmarshal(raw, &data); err != nil {
		return sloSessionData{}, fmt.Errorf("failed to unmarshal cookie value: %w", err)
	}

	if data.NameID == "" {
		return sloSessionData{}, errors.New("cookie has no name ID")
	}

	if time.Since(time.Unix(data.IssuedAt, 0)) > sloSessionCookieTTL {
		return sloSessionData{}, errors.New("cookie is expired")
	}

	return data, nil
}

func nameIDCookieMAC(raw, key []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(raw)

	return h.Sum(nil)
}

func readMetadata(cfg *specs.AuthConfigSpec_SAML) (*saml.EntityDescriptor, error) {
	if cfg.Url != "" {
		idpMetadataURL, err := url.Parse(cfg.Url)
		if err != nil {
			return nil, err
		}

		return samlsp.FetchMetadata(context.Background(), http.DefaultClient,
			*idpMetadataURL)
	}

	data, err := os.ReadFile(cfg.Metadata)
	if err != nil {
		return nil, err
	}

	return samlsp.ParseMetadata(data)
}

func createErrorHandler(logger *zap.Logger, advertisedURL string) func(http.ResponseWriter, *http.Request, error) {
	logger = logger.With(logging.Component("saml"))

	return func(w http.ResponseWriter, r *http.Request, err error) {
		if invalidSAML, ok := errors.AsType[*saml.InvalidResponseError](err); ok {
			// When the IdP sends a LogoutResponse to the ACS endpoint (e.g., because the
			// IdP's SAML client has no dedicated SLO URL configured), the ACS handler fails
			// to parse it as a login Response. Treat this as a completed logout.
			//
			// NOTE: this relies on the crewjam/saml library including "LogoutResponse" in the
			// error message when it encounters a LogoutResponse instead of an expected Response.
			// There is no structured way to detect this; if the library changes its error format,
			// this detection may need updating.
			if strings.Contains(invalidSAML.PrivateErr.Error(), "LogoutResponse") {
				logger.Info(
					"received LogoutResponse on ACS endpoint, treating as logout",
					zap.Error(invalidSAML.PrivateErr),
				)

				deleteNameIDCookie(w)

				http.Redirect(w, r, advertisedURL, http.StatusSeeOther)

				return
			}

			logger.Warn(
				"received invalid saml response",
				zap.String("response", invalidSAML.Response),
				zap.Time("now", invalidSAML.Now),
				zap.Error(invalidSAML.PrivateErr),
			)
		} else {
			logger.Error("saml error", zap.Error(err))
		}

		http.Redirect(w, r, "/forbidden", http.StatusSeeOther)
	}
}
