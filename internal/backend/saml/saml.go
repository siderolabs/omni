// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package saml contains SAML setup handlers.
package saml

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/crewjam/saml"
	"github.com/crewjam/saml/samlsp"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/internal/backend/logging"
	"github.com/siderolabs/omni/internal/backend/monitoring"
)

// NameIDCookieName is the cookie used to store the SAML session data for SLO.
const NameIDCookieName = "saml_name_id"

// sloSessionData holds the SAML assertion fields needed to build a LogoutRequest.
type sloSessionData struct {
	NameID       string `json:"n"`
	Format       string `json:"f,omitempty"`
	SessionIndex string `json:"s,omitempty"`
}

// NewHandler creates new SAML handler.
func NewHandler(state state.State, cfg *specs.AuthConfigSpec_SAML, logger *zap.Logger, apiURL string) (*samlsp.Middleware, error) {
	idpMetadata, err := readMetadata(cfg)
	if err != nil {
		return nil, err
	}

	rootURL, err := url.Parse(apiURL)
	if err != nil {
		return nil, err
	}

	opts := samlsp.Options{
		URL:               *rootURL,
		IDPMetadata:       idpMetadata,
		LogoutBindings:    []string{saml.HTTPRedirectBinding, saml.HTTPPostBinding},
		AllowIDPInitiated: true,
	}

	serviceProvider := samlsp.DefaultServiceProvider(opts)

	if cfg.NameIdFormat != "" {
		serviceProvider.AuthnNameIDFormat = saml.NameIDFormat(cfg.NameIdFormat)
	}

	requestTracker := samlsp.DefaultRequestTracker(opts, &serviceProvider)
	requestTracker.Codec = &Encoder{}

	m := &samlsp.Middleware{
		ServiceProvider: serviceProvider,
		ResponseBinding: saml.HTTPPostBinding,
		OnError:         createErrorHandler(logger, apiURL),
		Session: NewSessionProvider(
			state,
			requestTracker,
			logger.With(logging.Component("saml_session")),
			cfg.AttributeRules,
		),
		RequestTracker:   requestTracker,
		AssertionHandler: samlsp.DefaultAssertionHandler(samlsp.Options{}),
	}

	return m, nil
}

// RegisterHandlers adds SAML handlers for ACS, metadata, and SLO.
func RegisterHandlers(m *samlsp.Middleware, mux *http.ServeMux, logger *zap.Logger, advertisedURL string) {
	logger = logger.With(zap.String("handler", "saml"))

	md := http.HandlerFunc(m.ServeMetadata)
	promLabel := prometheus.Labels{"handler": "saml"}

	mux.Handle("/saml/", monitoring.NewHandler(
		logging.NewHandler(m, logger),
		promLabel,
	))

	mux.Handle("/saml/metadata", monitoring.NewHandler(
		logging.NewHandler(md, logger),
		promLabel,
	))

	mux.Handle("/saml/slo", monitoring.NewHandler(
		logging.NewHandler(createSLOHandler(m, advertisedURL, logger), logger),
		promLabel,
	))
}

// CreateLogoutHandler returns an HTTP handler that performs SAML Single Logout.
// It reads the NameID, format, and session index from a cookie set during login,
// builds a LogoutRequest, and redirects to the IdP's SLO endpoint. If the IdP
// has no SLO endpoint or no cookie is present, it falls back to a local-only logout.
func CreateLogoutHandler(m *samlsp.Middleware, advertisedURL string, logger *zap.Logger) http.HandlerFunc {
	logger = logger.With(logging.Component("saml_logout"))

	return func(w http.ResponseWriter, r *http.Request) {
		data, ok := readNameIDCookie(r)

		deleteNameIDCookie(w)

		if !ok {
			logger.Debug("no SAML SLO cookie, skipping SLO")

			http.Redirect(w, r, advertisedURL, http.StatusSeeOther)

			return
		}

		sloURL := m.ServiceProvider.GetSLOBindingLocation(saml.HTTPRedirectBinding)
		if sloURL == "" {
			logger.Debug("IdP does not advertise SLO redirect endpoint, skipping SLO")

			http.Redirect(w, r, advertisedURL, http.StatusSeeOther)

			return
		}

		req, err := m.ServiceProvider.MakeLogoutRequest(sloURL, data.NameID)
		if err != nil {
			logger.Error("failed to build SAML logout request", zap.Error(err))

			http.Redirect(w, r, advertisedURL, http.StatusSeeOther)

			return
		}

		if data.Format != "" && req.NameID != nil {
			req.NameID.Format = data.Format
		}

		if data.SessionIndex != "" {
			req.SessionIndex = &saml.SessionIndex{Value: data.SessionIndex}
		}

		// Note: for HTTP-Redirect binding the IdP ignores embedded XML signatures.
		// MakeLogoutRequest may have added one, but it's harmless — the IdP validates
		// the query-string signature (SigAlg + Signature params), not the XML body signature.
		// We intentionally skip re-signing after modifying Format/SessionIndex.
		// See SAML 2.0 Bindings, §3.4.4.1 (HTTP-Redirect DEFLATE Encoding), which
		// defines signatures over the URL query parameters, not the XML payload.

		redirectURL := req.Redirect(advertisedURL)

		http.Redirect(w, r, redirectURL.String(), http.StatusFound)
	}
}

func createSLOHandler(m *samlsp.Middleware, advertisedURL string, logger *zap.Logger) http.HandlerFunc {
	logger = logger.With(logging.Component("saml_slo"))

	return func(w http.ResponseWriter, r *http.Request) {
		if err := m.ServiceProvider.ValidateLogoutResponseRequest(r); err != nil {
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

func readNameIDCookie(r *http.Request) (sloSessionData, bool) {
	cookie, err := r.Cookie(NameIDCookieName)
	if err != nil || cookie.Value == "" {
		return sloSessionData{}, false
	}

	raw, err := base64.URLEncoding.DecodeString(cookie.Value)
	if err != nil {
		return sloSessionData{}, false
	}

	var data sloSessionData
	if err := json.Unmarshal(raw, &data); err != nil || data.NameID == "" {
		return sloSessionData{}, false
	}

	return data, true
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
		var invalidSAML *saml.InvalidResponseError
		if errors.As(err, &invalidSAML) {
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
