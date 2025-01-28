// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package virtual

import (
	"context"
	"fmt"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/panichandler"
)

// Computed is a virtual state implementation which provides virtual resources which are computed on the fly
// and cached in the in inmem.State.
type Computed struct {
	state          state.State
	watchScheduler *DedupScheduler
	activeWatches  *prometheus.GaugeVec
	resolveID      ProducerIDTransformer
	logger         *zap.Logger
}

// ProducerIDTransformer maps the incoming resource id into some other id.
type ProducerIDTransformer func(context.Context, resource.Pointer) (resource.Pointer, error)

// NoTransform passes the resource pointer without any change.
func NoTransform(_ context.Context, ptr resource.Pointer) (resource.Pointer, error) {
	return ptr, nil
}

// NewComputed creates new computed state.
func NewComputed(resourceType string, factory ProducerFactory, resolveID ProducerIDTransformer, cleanupInterval time.Duration, logger *zap.Logger) *Computed {
	state := state.WrapCore(namespaced.NewState(inmem.Build))
	scheduler := NewDedupScheduler(resourceType, state, factory, cleanupInterval, logger)

	prometheus.DefaultRegisterer.MustRegister(scheduler)

	return &Computed{
		state:          state,
		watchScheduler: scheduler,
		resolveID:      resolveID,
		activeWatches: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "omni_virtual_state_watches",
			Help: "Number of virtual state compute watches.",
		}, []string{
			"type",
			"id",
		}),
		logger: logger,
	}
}

// Run starts underlying watch scheduler.
func (st *Computed) Run(ctx context.Context) {
	st.watchScheduler.Run(ctx)
}

// Get implements state.State.
func (st *Computed) Get(ctx context.Context, ptr resource.Pointer, _ ...state.GetOption) (resource.Resource, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	watchCh := make(chan state.Event)

	if err := st.Watch(ctx, ptr, watchCh); err != nil {
		return nil, err
	}

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case ev := <-watchCh:
			//nolint:exhaustive
			switch ev.Type {
			case state.Created:
				return ev.Resource, nil
			case state.Destroyed:
				// ignore
			case state.Errored:
				return nil, fmt.Errorf("watch errored: %w", ev.Error)
			default:
				return nil, fmt.Errorf("unexpected event %s", ev.Type.String())
			}
		}
	}
}

// Watch implements state.State.
func (st *Computed) Watch(ctx context.Context, ptr resource.Pointer, c chan<- state.Event, opts ...state.WatchOption) error {
	ptr, err := st.resolveID(ctx, ptr)
	if err != nil {
		return fmt.Errorf("failed to resolve watch id: %w", err)
	}

	activeWatches := st.activeWatches.With(prometheus.Labels{"type": ptr.Type(), "id": ptr.ID()})

	if err = st.watchScheduler.Start(ctx, ptr); err != nil {
		return fmt.Errorf("failed to start watch: %w", err)
	}

	err = st.state.Watch(ctx, ptr, c, opts...)
	if err != nil {
		st.watchScheduler.Stop(ptr)

		return fmt.Errorf("failed to watch inmem state: %w", err)
	}

	activeWatches.Inc()

	panichandler.Go(func() {
		<-ctx.Done()

		activeWatches.Dec()

		st.watchScheduler.Stop(ptr)
	}, st.logger)

	return nil
}

// Describe implements prom.Collector interface.
func (st *Computed) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(st, ch)
}

// Collect implements prom.Collector interface.
func (st *Computed) Collect(ch chan<- prometheus.Metric) {
	st.activeWatches.Collect(ch)
}

var _ prometheus.Collector = &DedupScheduler{}
