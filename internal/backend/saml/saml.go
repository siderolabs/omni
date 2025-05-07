// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package saml contains SAML setup handlers.
package saml

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"os"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/crewjam/saml"
	"github.com/crewjam/saml/samlsp"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/internal/backend/logging"
	"github.com/siderolabs/omni/internal/backend/monitoring"
	"github.com/siderolabs/omni/internal/pkg/config"
)

// NewHandler creates new SAML handler.
func NewHandler(state state.State, cfg *specs.AuthConfigSpec_SAML, logger *zap.Logger) (*samlsp.Middleware, error) {
	idpMetadata, err := readMetadata(cfg)
	if err != nil {
		return nil, err
	}

	rootURL, err := url.Parse(config.Config.APIURL)
	if err != nil {
		return nil, err
	}

	opts := samlsp.Options{
		URL:               *rootURL,
		IDPMetadata:       idpMetadata,
		LogoutBindings:    []string{saml.HTTPPostBinding},
		AllowIDPInitiated: true,
	}

	serviceProvider := samlsp.DefaultServiceProvider(opts)
	requestTracker := samlsp.DefaultRequestTracker(opts, &serviceProvider)
	requestTracker.Codec = &Encoder{}

	m := &samlsp.Middleware{
		ServiceProvider: serviceProvider,
		ResponseBinding: saml.HTTPPostBinding,
		OnError:         createErrorHandler(logger),
		Session: NewSessionProvider(
			state,
			requestTracker,
			logger.With(logging.Component("saml_session")),
		),
		RequestTracker:   requestTracker,
		AssertionHandler: samlsp.DefaultAssertionHandler(samlsp.Options{}),
	}

	return m, nil
}

// RegisterHandlers adds login and logout handlers.
func RegisterHandlers(saml *samlsp.Middleware, mux *http.ServeMux, logger *zap.Logger) {
	logger = logger.With(zap.String("handler", "saml"))
	login := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		saml.HandleStartAuthFlow(w, r)
	})

	md := http.HandlerFunc(saml.ServeMetadata)
	promLabel := prometheus.Labels{"handler": "saml"}

	mux.Handle("/saml/", monitoring.NewHandler(
		logging.NewHandler(saml, logger),
		promLabel,
	))

	mux.Handle("/saml/metadata", monitoring.NewHandler(
		logging.NewHandler(md, logger),
		promLabel,
	))

	mux.Handle("/login", monitoring.NewHandler(
		logging.NewHandler(login, logger),
		promLabel,
	))
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

func createErrorHandler(logger *zap.Logger) func(http.ResponseWriter, *http.Request, error) {
	logger = logger.With(logging.Component("saml"))

	return func(w http.ResponseWriter, r *http.Request, err error) {
		var invalidSAML *saml.InvalidResponseError

		if errors.As(err, &invalidSAML) {
			logger.Warn("received invalid saml response",
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
