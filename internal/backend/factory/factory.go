// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package factory provides the code to call Talos image factory.
package factory

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/pkg/config"
)

// Handler of image requests.
type Handler struct {
	state  state.State
	logger *zap.Logger
	config *config.Registries
}

// NewHandler creates a new factory proxy handler.
func NewHandler(state state.State, logger *zap.Logger, config *config.Registries) *Handler {
	return &Handler{
		state:  state,
		logger: logger,
		config: config,
	}
}

func setContentHeaders(w http.ResponseWriter, contentType, filename string) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", "attachment; filename="+filename)
	w.Header().Set("Access-Control-Expose-Headers", "Content-Disposition")
}

func httpNotFound(w http.ResponseWriter) {
	http.Error(w, "Not found", http.StatusNotFound)
}

func (handler *Handler) handleError(msg string, w http.ResponseWriter, err error) {
	handler.logger.Error(msg, zap.Error(err))

	switch status.Code(err) { //nolint:exhaustive
	case codes.Unauthenticated:
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	case codes.PermissionDenied:
		http.Error(w, "Permission denied", http.StatusForbidden)
	default:
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// ServeHTTP handles image requests.
func (handler *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	io.Copy(io.Discard, r.Body) //nolint:errcheck
	r.Body.Close()              //nolint:errcheck

	switch r.Method {
	case http.MethodGet, http.MethodHead:
		// supported
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)

		return
	}

	params, err := parseRequest(r, handler.state, handler.config)
	if err != nil {
		if errors.Is(err, errNotFound) {
			httpNotFound(w)

			return
		}

		handler.handleError("failed to parse request", w, err)

		return
	}

	handler.logger.Info("proxy request", zap.String("url", params.ProxyURL))

	proxyReq, err := http.NewRequestWithContext(r.Context(), r.Method, params.ProxyURL, nil)
	if err != nil {
		handler.handleError("failed to call image factory", w, err)

		return
	}

	setContentHeaders(w, params.ContentType, params.DestinationFilename)

	for name, values := range r.Header {
		for _, value := range values {
			proxyReq.Header.Add(name, value)
		}
	}

	client := http.Client{}

	resp, err := client.Do(proxyReq)
	if err != nil {
		handler.handleError("failed to call image factory", w, err)

		return
	}

	defer resp.Body.Close() //nolint:errcheck

	for name, values := range resp.Header {
		// ignore content disposition header from the image factory
		// as we override it here
		if name == "Content-Disposition" {
			continue
		}

		for _, value := range values {
			w.Header().Add(name, value)
		}
	}

	w.WriteHeader(resp.StatusCode)

	io.Copy(w, resp.Body) //nolint:errcheck
}

var errNotFound = errors.New("not found")

// ProxyParams is exposed for the unit tests.
type ProxyParams struct {
	ProxyURL            string
	ContentType         string
	DestinationFilename string
}

func parseRequest(r *http.Request, st state.State, config *config.Registries) (*ProxyParams, error) {
	segments := strings.Split(strings.Trim(r.URL.Path, "/"), "/")

	if len(segments) < 4 {
		return nil, errNotFound
	}

	if segments[0] != "image" {
		return nil, errNotFound
	}

	ctx := r.Context()

	media, err := safe.ReaderGet[*omni.InstallationMedia](ctx, st, resource.NewMetadata(
		resources.EphemeralNamespace, omni.InstallationMediaType, segments[3], resource.VersionUndefined,
	))
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, errNotFound
		}

		return nil, err
	}

	secureBoot := r.URL.Query().Get(constants.SecureBoot) == "true"

	talosVersion := segments[2]

	srcFilename := talosVersion

	if secureBoot {
		srcFilename += "-secureboot"
	}

	p := &ProxyParams{
		ContentType:         media.TypedSpec().Value.ContentType,
		DestinationFilename: fmt.Sprintf("%s-%s.%s", media.TypedSpec().Value.DestFilePrefix, srcFilename, media.TypedSpec().Value.Extension),
	}

	proxyURL, err := url.Parse(config.GetImageFactoryBaseURL())
	if err != nil {
		return nil, err
	}

	filename := media.TypedSpec().Value.GenerateFilename(!quirks.New(talosVersion).SupportsOverlay(), secureBoot, true)

	// strip the last element from the URL
	// replace it with the generated filename
	segments = append(segments[:3], filename)

	proxyURL = proxyURL.JoinPath(segments...)

	p.ProxyURL = proxyURL.String()

	return p, nil
}
