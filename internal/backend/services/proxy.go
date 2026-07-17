// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package services

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/siderolabs/omni/internal/pkg/config"
)

// NewFrontendHandler creates a new frontend handler from the given dst address. If dst is empty, the handler will be a no-op.
func NewFrontendHandler(config config.DevServerProxyService, logger *zap.Logger) (http.Handler, error) {
	proxyTo := config.GetProxyTo()
	if proxyTo == "" {
		logger.Info("frontend handler disabled")

		return nopHandler, nil
	}

	parse, err := url.ParseRequestURI(proxyTo)
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

	logger.Info("frontend handler enabled", zap.String("destination", proxyTo))

	return proxy, nil
}

var nopHandler http.Handler = &nopServeHTTP{}

type nopServeHTTP struct{}

func (*nopServeHTTP) ServeHTTP(http.ResponseWriter, *http.Request) {}
