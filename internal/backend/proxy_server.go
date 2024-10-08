// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package backend

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
)

// Proxy is a proxy server.
type Proxy interface {
	Run(ctx context.Context, next http.Handler, logger *zap.Logger) error
}

// NewProxyServer creates a new proxy server. If the destination is empty, the proxy server will be a no-op.
func NewProxyServer(bindAddr string, proxyTo http.Handler, keyFile, certFile string) Proxy {
	switch {
	case bindAddr == "":
		return &nopProxy{reason: "bind address is empty"}
	case proxyTo == nopHandler:
		return &nopProxy{reason: "proxy destination is empty"}
	default:
		return &httpProxy{
			bindAddr: bindAddr,
			proxyTo:  proxyTo,
			keyFile:  keyFile,
			certFile: certFile,
		}
	}
}

type httpProxy struct {
	proxyTo  http.Handler
	bindAddr string
	keyFile  string
	certFile string
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
	srv := &server{
		server: &http.Server{
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if hasPrefix(r.URL.Path, "/api/", "/omnictl/", "/talosctl/", "/image/") {
					next.ServeHTTP(w, r)

					return
				}

				prx.proxyTo.ServeHTTP(w, r)
			}),
			Addr: prx.bindAddr,
		},
		certData: &certData{
			certFile: prx.certFile,
			keyFile:  prx.keyFile,
		},
	}

	logger = logger.With(zap.String("server", prx.bindAddr), zap.String("server_type", "proxy_server"))

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
func NewFrontendHandler(dst string, logger *zap.Logger) (http.Handler, error) {
	if dst == "" {
		logger.Info("frontend handler disabled")

		return nopHandler, nil
	}

	parse, err := url.ParseRequestURI(dst)
	if err != nil {
		return nil, fmt.Errorf("failed to parse proxy destination: %w", err)
	}

	proxy := httputil.NewSingleHostReverseProxy(parse)

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

	logger.Info("frontend handler enabled", zap.String("destination", dst))

	return proxy, nil
}

var nopHandler http.Handler = &nopServeHTTP{}

type nopServeHTTP struct{}

func (*nopServeHTTP) ServeHTTP(http.ResponseWriter, *http.Request) {}
