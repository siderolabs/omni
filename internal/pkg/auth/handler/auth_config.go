// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package handler

import (
	"net/http"

	"go.uber.org/zap"

	"github.com/siderolabs/omni/internal/pkg/auth"
	"github.com/siderolabs/omni/internal/pkg/ctxstore"
)

// AuthConfig represents the configuration for the auth config interceptor.
type AuthConfig struct {
	next    http.Handler
	logger  *zap.Logger
	enabled bool
}

// NewAuthConfig returns a new auth config interceptor.
func NewAuthConfig(handler http.Handler, enabled bool, logger *zap.Logger) *AuthConfig {
	return &AuthConfig{
		next:    handler,
		enabled: enabled,
		logger:  logger,
	}
}

func (c *AuthConfig) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	ctx := ctxstore.WithValue(request.Context(), auth.EnabledAuthContextKey{Enabled: c.enabled})
	request = request.WithContext(ctx)

	c.next.ServeHTTP(writer, request)
}
