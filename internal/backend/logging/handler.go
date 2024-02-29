// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package logging

import (
	"net"
	"net/http"

	"github.com/felixge/httpsnoop"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	grpc_ctxtags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Handler adds structured logging to each request going through a wrapped handler.
type Handler struct {
	h      http.Handler
	logger *zap.Logger
	fields []zap.Field
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	remoteAddr := r.RemoteAddr
	remoteAddr, _, _ = net.SplitHostPort(remoteAddr) //nolint:errcheck

	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		remoteAddr = realIP
	}

	logger := h.logger.With(
		zap.String("request_url", r.RequestURI),
		zap.String("method", r.Method),
		zap.String("remote_addr", remoteAddr),
	).With(h.fields...)

	// inject empty ctxtags and logger into request context
	r = r.WithContext(
		ctxzap.ToContext(
			grpc_ctxtags.SetInContext(r.Context(), grpc_ctxtags.NewTags()),
			logger,
		),
	)

	metrics := httpsnoop.CaptureMetrics(h.h, w, r)

	// get injected ctxtags back
	ctxtags := grpc_ctxtags.Extract(r.Context()).Values()
	fields := make([]zapcore.Field, 0, len(ctxtags))

	for k, v := range ctxtags {
		fields = append(fields, zap.Any(k, v))
	}

	logger.Info("HTTP request done",
		append(
			[]zapcore.Field{
				zap.Duration("duration", metrics.Duration),
				zap.Int("status", metrics.Code),
				zap.Int64("response_length", metrics.Written),
			},
			fields...,
		)...,
	)
}

// NewHandler creates new Handler.
func NewHandler(h http.Handler, logger *zap.Logger) *Handler {
	return &Handler{h: h, logger: logger}
}
