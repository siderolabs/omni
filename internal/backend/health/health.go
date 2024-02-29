// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package health defines health check endpoints for omni
package health

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/pkg/auth/actor"
)

// Handler of health requests.
type Handler struct {
	state  state.State
	ptr    resource.Pointer
	logger *zap.Logger
}

// NewHandler creates http handler wrapper which reports health status.
func NewHandler(state state.State, logger *zap.Logger) http.Handler {
	return &Handler{
		state:  state,
		ptr:    omni.NewTalosVersion(resources.DefaultNamespace, "health-check").Metadata(),
		logger: logger,
	}
}

// ServeHTTP handles health check requests.
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

	switch r.URL.Path {
	case "/", "/healthz":
		ctx, cancel := context.WithTimeout(actor.MarkContextAsInternalActor(r.Context()), 1*time.Second)
		defer cancel()

		if _, err := handler.state.Get(ctx, handler.ptr); !state.IsNotFoundError(err) {
			handler.logger.Error("unexpected error on health check", zap.Error(err))

			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	default:
		http.Error(w, "Not found", http.StatusNotFound)
	}
}
