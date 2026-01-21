// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package services

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/siderolabs/omni/internal/pkg/config"
)

// Proxy is a proxy server.
type Proxy interface {
	Run(ctx context.Context, next http.Handler, logger *zap.Logger) error
}

// NewProxy creates a new proxy server. If the destination is empty, the proxy server will be a no-op.
func NewProxy(config *config.DevServerProxyService, handler http.Handler) Proxy {
	switch {
	case config == nil:
		return &nopProxy{reason: "dev proxy is disabled"}
	case config.BindEndpoint == "":
		return &nopProxy{reason: "bind address is empty"}
	case config.ProxyTo == "":
		return &nopProxy{reason: "proxy destination is empty"}
	default:
		return &httpProxy{
			config:  config,
			proxyTo: handler,
		}
	}
}

type httpProxy struct {
	proxyTo http.Handler
	config  *config.DevServerProxyService
}

func hasPrefix(s string, prefixes ...string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(s, prefix) {
			return true
		}
	}

	return false
}

func (prx *httpProxy) Run(ctx context.Context, next http.Handler, logger *zap.Logger) error {
	srv := NewFromConfig(
		prx.config,
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if hasPrefix(r.URL.Path, "/api/", "/omnictl/", "/talosctl/", "/image/") {
				next.ServeHTTP(w, r)

				return
			}

			prx.proxyTo.ServeHTTP(w, r)
		}),
	)

	logger = logger.With(zap.String("server", prx.config.BindEndpoint), zap.String("server_type", "proxy_server"))

	return srv.Run(ctx, logger)
}

type nopProxy struct {
	reason string
}

func (n *nopProxy) Run(ctx context.Context, _ http.Handler, logger *zap.Logger) error {
	logger.Info("proxy server disabled", zap.String("reason", n.reason))

	<-ctx.Done()

	return nil
}

// NewFrontendHandler creates a new frontend handler from the given dst address. If dst is empty, the handler will be a no-op.
func NewFrontendHandler(config *config.DevServerProxyService, logger *zap.Logger) (http.Handler, error) {
	if config == nil || config.ProxyTo == "" {
		logger.Info("frontend handler disabled")

		return nopHandler, nil
	}

	parse, err := url.ParseRequestURI(config.ProxyTo)
	if err != nil {
		return nil, fmt.Errorf("failed to parse proxy destination: %w", err)
	}

	proxyErrorLogger, err := zap.NewStdLogAt(logger, zapcore.WarnLevel)
	if err != nil {
		return nil, fmt.Errorf("failed to create proxy error logger: %w", err)
	}

	proxy := httputil.NewSingleHostReverseProxy(parse)
	proxy.ErrorLog = proxyErrorLogger

	// Some basic client settings so that the proxy works as expected even in dev mode.
	proxy.Transport = &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		IdleConnTimeout:       90 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	logger.Info("frontend handler enabled", zap.String("destination", config.ProxyTo))

	return proxy, nil
}

var nopHandler http.Handler = &nopServeHTTP{}

type nopServeHTTP struct{}

func (*nopServeHTTP) ServeHTTP(http.ResponseWriter, *http.Request) {}
