// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/prometheus/client_golang/prometheus"
)

// stateMetrics wraps COSI core state and produces metrics on resource access.
type stateMetrics struct {
	st state.CoreState

	resourceOperations *prometheus.CounterVec
	resourceThroughput *prometheus.CounterVec
}

// Check interfaces.
var (
	_ prometheus.Collector = &stateMetrics{}
	_ state.CoreState      = &stateMetrics{}
)

func wrapStateWithMetrics(st state.CoreState) *stateMetrics {
	return &stateMetrics{
		st: st,

		resourceOperations: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "omni_resource_operations_total",
				Help: "Number of resource operations by operation and resource type.",
			},
			[]string{"operation", "type"},
		),
		resourceThroughput: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "omni_resource_throughput_total",
				Help: "Number of resources processed by watches/reads/writes.",
			},
			[]string{"kind", "type"},
		),
	}
}

// Describe implements prom.Collector interface.
func (metrics *stateMetrics) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(metrics, ch)
}

// Collect implements prom.Collector interface.
func (metrics *stateMetrics) Collect(ch chan<- prometheus.Metric) {
	metrics.resourceOperations.Collect(ch)
	metrics.resourceThroughput.Collect(ch)
}

func (metrics *stateMetrics) Get(ctx context.Context, r resource.Pointer, opts ...state.GetOption) (resource.Resource, error) {
	metrics.resourceOperations.WithLabelValues("get", r.Type()).Inc()

	result, err := metrics.st.Get(ctx, r, opts...)

	if result != nil {
		metrics.resourceThroughput.WithLabelValues("read", r.Type()).Inc()
	}

	return result, err
}

func (metrics *stateMetrics) List(ctx context.Context, r resource.Kind, opts ...state.ListOption) (resource.List, error) {
	metrics.resourceOperations.WithLabelValues("list", r.Type()).Inc()

	result, err := metrics.st.List(ctx, r, opts...)

	metrics.resourceThroughput.WithLabelValues("read", r.Type()).Add(float64(len(result.Items)))

	return result, err
}

func (metrics *stateMetrics) Create(ctx context.Context, r resource.Resource, opts ...state.CreateOption) error {
	metrics.resourceOperations.WithLabelValues("create", r.Metadata().Type()).Inc()

	err := metrics.st.Create(ctx, r, opts...)

	if err == nil {
		metrics.resourceThroughput.WithLabelValues("write", r.Metadata().Type()).Inc()
	}

	return err
}

func (metrics *stateMetrics) Update(ctx context.Context, newResource resource.Resource, opts ...state.UpdateOption) error {
	metrics.resourceOperations.WithLabelValues("update", newResource.Metadata().Type()).Inc()

	err := metrics.st.Update(ctx, newResource, opts...)

	if err == nil {
		metrics.resourceThroughput.WithLabelValues("write", newResource.Metadata().Type()).Inc()
	}

	return err
}

func (metrics *stateMetrics) Destroy(ctx context.Context, r resource.Pointer, opts ...state.DestroyOption) error {
	metrics.resourceOperations.WithLabelValues("destroy", r.Type()).Inc()

	err := metrics.st.Destroy(ctx, r, opts...)

	if err == nil {
		metrics.resourceThroughput.WithLabelValues("write", r.Type()).Inc()
	}

	return err
}

func (metrics *stateMetrics) Watch(ctx context.Context, r resource.Pointer, ch chan<- state.Event, opts ...state.WatchOption) error {
	metrics.resourceOperations.WithLabelValues("watch", r.Type()).Inc()

	return metrics.st.Watch(ctx, r, metrics.watchChannelWrapper(ctx, ch, r.Type()), opts...)
}

func (metrics *stateMetrics) WatchKind(ctx context.Context, r resource.Kind, ch chan<- state.Event, opts ...state.WatchKindOption) error {
	metrics.resourceOperations.WithLabelValues("watch", r.Type()).Inc()

	return metrics.st.WatchKind(ctx, r, metrics.watchChannelWrapper(ctx, ch, r.Type()), opts...)
}

func (metrics *stateMetrics) WatchKindAggregated(ctx context.Context, r resource.Kind, c chan<- []state.Event, opts ...state.WatchKindOption) error {
	metrics.resourceOperations.WithLabelValues("watch", r.Type()).Inc()

	return metrics.st.WatchKindAggregated(ctx, r, metrics.watchAggregatedChannelWrapper(ctx, c, r.Type()), opts...)
}

func (metrics *stateMetrics) watchChannelWrapper(ctx context.Context, out chan<- state.Event, typ resource.Type) chan<- state.Event {
	in := make(chan state.Event)

	go func() {
		for {
			var ev state.Event

			select {
			case <-ctx.Done():
				return
			case ev = <-in:
				metrics.resourceThroughput.WithLabelValues("watch", typ).Inc()
			}

			select {
			case <-ctx.Done():
				return
			case out <- ev:
			}
		}
	}()

	return in
}

func (metrics *stateMetrics) watchAggregatedChannelWrapper(ctx context.Context, out chan<- []state.Event, typ resource.Type) chan<- []state.Event {
	in := make(chan []state.Event)

	go func() {
		for {
			var ev []state.Event

			select {
			case <-ctx.Done():
				return
			case ev = <-in:
				metrics.resourceThroughput.WithLabelValues("watch", typ).Add(float64(len(ev)))
			}

			select {
			case <-ctx.Done():
				return
			case out <- ev:
			}
		}
	}()

	return in
}
