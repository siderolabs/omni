// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package k8sproxy

import (
	"net/http"
	"net/http/httputil"

	"go.uber.org/zap"

	"github.com/siderolabs/omni/internal/backend/logging"
	"github.com/siderolabs/omni/internal/pkg/ctxstore"
	"github.com/siderolabs/omni/internal/pkg/grpcutil/grpczap/ctxzap"
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
		Rewrite:   p.rewrite,
		Transport: m,
		ErrorLog:  zap.NewStdLog(logger),
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			ctxzap.Error(r.Context(), "proxy handling error", zap.Error(err))

			http.Error(w, "proxy error", http.StatusBadGateway)
		},
	}

	return p
}

// rewrite sets the target URL and the headers for the reverse proxy.
func (p *proxyHandler) rewrite(req *httputil.ProxyRequest) {
	// read from the inbound request: a header the client lists in Connection is gone from the outbound one
	req.Out.Header = forwardHeaders(req.Out.Header, req.In.Header)

	req.SetXForwarded()

	clusterNameVal, ok := ctxstore.Value[clusterContextKey](req.In.Context())
	if !ok {
		ctxzap.Error(req.In.Context(), "cluster name not found in request context")

		return
	}

	connector, err := p.multiplexer.getClusterConnector(req.In.Context(), clusterNameVal.ClusterName)
	if err != nil {
		ctxzap.Error(req.In.Context(), "failed to get cluster connector", zap.Error(err))

		return
	}

	req.Out.URL.Scheme = "https"
	req.Out.URL.Host = connector.apiHost
}

func (p *proxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.proxy.ServeHTTP(w, r)
}
