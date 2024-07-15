// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package k8sproxy

import (
	"net/http"
	"net/http/httputil"

	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/internal/backend/logging"
	"github.com/siderolabs/omni/internal/pkg/ctxstore"
)

// proxyHandler implements the HTTP reverse proxy.
type proxyHandler struct {
	multiplexer *multiplexer
	proxy       *httputil.ReverseProxy
}

func newProxyHandler(m *multiplexer, logger *zap.Logger) *proxyHandler {
	p := &proxyHandler{
		multiplexer: m,
	}

	logger = logger.With(logging.Component("k8s_proxy"))

	p.proxy = &httputil.ReverseProxy{
		Director:  p.director,
		Transport: m,
		ErrorLog:  zap.NewStdLog(logger),
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			ctxzap.Error(r.Context(), "proxy handling error", zap.Error(err))

			http.Error(w, "proxy error", http.StatusBadGateway)
		},
	}

	return p
}

// director sets the target URL for the reverse proxy.
func (p *proxyHandler) director(req *http.Request) {
	clusterNameVal, ok := ctxstore.Value[clusterContextKey](req.Context())
	if !ok {
		ctxzap.Error(req.Context(), "cluster name not found in request context")

		return
	}

	connector, err := p.multiplexer.getClusterConnector(req.Context(), clusterNameVal.ClusterName)
	if err != nil {
		ctxzap.Error(req.Context(), "failed to get cluster connector", zap.Error(err))

		return
	}

	req.URL.Scheme = "https"
	req.URL.Host = connector.apiHost
}

func (p *proxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.proxy.ServeHTTP(w, r)
}
