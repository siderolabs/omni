// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package monitoring defines monitoring helpers common for the various parts of the project.
package monitoring

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	requestsInFlightGauge prometheus.Gauge
	requestsCounter       *prometheus.CounterVec
	requestsDuration      *prometheus.HistogramVec
	requestsResponseSize  *prometheus.HistogramVec
)

func init() {
	requestsInFlightGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "http_requests_current",
		Help: "Current number of requests being processed.",
	})

	requestsCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Requests total.",
		},
		[]string{"code", "method", "handler"},
	)

	// requestsDuration is partitioned by the HTTP method and handler. It uses custom
	// buckets based on the expected request requestsDuration.
	requestsDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "A histogram of latencies for requests.",
			Buckets: []float64{.25, .5, 1, 2.5, 5, 10},
		},
		[]string{"handler", "method"},
	)

	// requestsResponseSize has no labels, making it a zero-dimensional
	// ObserverVec.
	requestsResponseSize = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_response_size_bytes",
			Help:    "A histogram of response sizes for requests.",
			Buckets: []float64{200, 500, 900, 1500},
		},
		[]string{"handler"},
	)

	// Register all of the metrics in the standard registry.
	prometheus.MustRegister(requestsInFlightGauge, requestsCounter, requestsDuration, requestsResponseSize)
}

// NewHandler creates http handler wrapper which reports metrics to prometheus.
func NewHandler(handler http.Handler, labels prometheus.Labels) http.Handler {
	return promhttp.InstrumentHandlerInFlight(requestsInFlightGauge,
		promhttp.InstrumentHandlerDuration(requestsDuration.MustCurryWith(labels),
			promhttp.InstrumentHandlerCounter(requestsCounter.MustCurryWith(labels),
				promhttp.InstrumentHandlerResponseSize(requestsResponseSize.MustCurryWith(labels), handler),
			),
		),
	)
}
