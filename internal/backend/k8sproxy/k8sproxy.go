// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package k8sproxy provides JWT-authenticated proxy to Kubernetes API server.
package k8sproxy

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/internal/backend/runtime"
	"github.com/siderolabs/omni/internal/backend/runtime/kubernetes"
)

// clusterContextKey is a type for cluster name.
type clusterContextKey struct{ ClusterName string }

// Handler implements the HTTP reverse proxy for Kubernetes clusters.
//
// Handler intercepts the request, verifies the authentication info and
// routes the request to matching Kubernetes cluster with impersonation headers.
//
// Handler itself implements httt.Handler interface.
type Handler struct {
	multiplexer *multiplexer
	chain       http.Handler
}

// NewHandler creates a new Handler.
func NewHandler(keyFunc KeyProvider, clusterUUIDResolver ClusterUUIDResolver, logger *zap.Logger) (*Handler, error) {
	multiplexer := newMultiplexer()
	proxy := newProxyHandler(multiplexer, logger)

	handler := &Handler{
		multiplexer: multiplexer,
		chain:       AuthorizeRequest(proxy, keyFunc, clusterUUIDResolver),
	}

	type kubeRuntime interface {
		RegisterCleanuper(kubernetes.ClusterCleanuper)
	}

	r, err := runtime.LookupInterface[kubeRuntime](kubernetes.Name)
	if err != nil {
		return nil, err
	}

	// register self as  cluster cleanuper
	r.RegisterCleanuper(handler)

	return handler, nil
}

// ServeHTTP implements http.Handler interface.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.chain.ServeHTTP(w, r)
}

// ClusterRemove implements ClusterCleanuper interface.
func (h *Handler) ClusterRemove(clusterName string) {
	h.multiplexer.removeClusterConnector(clusterName)
}

// Describe implements prom.Collector interface.
func (h *Handler) Describe(ch chan<- *prometheus.Desc) {
	h.multiplexer.Describe(ch)
}

// Collect implements prom.Collector interface.
func (h *Handler) Collect(ch chan<- prometheus.Metric) {
	h.multiplexer.Collect(ch)
}

var _ prometheus.Collector = &Handler{}
