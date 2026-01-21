// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package virtual

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"

	"github.com/siderolabs/omni/internal/backend/logging"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/virtual/pkg/producers"
)

// ProducerFactory is a function that creates a producer.
type ProducerFactory func(ctx context.Context, state state.State, ptr resource.Pointer, logger *zap.Logger) (producers.Producer, error)

type producerWrapper struct {
	producer producers.Producer
	resource resource.Pointer
	inuse    int
	stale    bool
}

// DedupScheduler handles computed watch state producers.
// It cleans up unused producers after defined interval
// and deduplicates producers identified by the resource IDs.
type DedupScheduler struct {
	producers       map[resource.ID]*producerWrapper
	logger          *zap.Logger
	state           state.State
	factory         ProducerFactory
	activeProducers *prometheus.GaugeVec
	sf              singleflight.Group
	resourceType    string
	producersMu     sync.Mutex
	cleanupInterval time.Duration
}

// NewDedupScheduler creates new dedup scheduler instance.
func NewDedupScheduler(resourceType string, state state.State, factory ProducerFactory, cleanupInterval time.Duration, logger *zap.Logger) *DedupScheduler {
	return &DedupScheduler{
		state:           state,
		factory:         factory,
		cleanupInterval: cleanupInterval,
		logger:          logger.With(logging.Component("compute_watch_scheduler")),
		producers:       map[string]*producerWrapper{},
		resourceType:    resourceType,

		activeProducers: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "omni_virtual_state_producers",
			Help: "Number of virtual state compute producers.",
		}, []string{
			"type",
		}),
	}
}

// Run starts cleanup goroutine.
// When stopped it shuts down all running producers.
func (d *DedupScheduler) Run(ctx context.Context) {
	ticker := time.NewTicker(d.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		var producersToStop []*producerWrapper

		d.producersMu.Lock()

		for id, pw := range d.producers {
			if pw.stale {
				d.logger.Debug("stopping producer",
					zap.String("type", pw.resource.Type()),
					zap.String("namespace", pw.resource.Namespace()),
					zap.String("id", pw.resource.ID()),
				)

				producersToStop = append(producersToStop, pw)

				pw.producer.Stop()

				if err := d.state.Destroy(ctx, pw.resource); err != nil && !state.IsNotFoundError(err) {
					d.logger.Error("failed to cleanup resources after stopping computed watch producer",
						zap.String("resource", fmt.Sprintf("%s/%s/%s", pw.resource.Namespace(), pw.resource.Type(), pw.resource.ID())),
					)
				}

				delete(d.producers, id)
			} else if pw.inuse == 0 {
				d.producers[id].stale = true
			}
		}

		d.producersMu.Unlock()

		for _, pw := range producersToStop {
			pw.producer.Cleanup()
		}
	}
}

func (d *DedupScheduler) ref(ptr resource.Pointer) bool {
	d.producersMu.Lock()
	defer d.producersMu.Unlock()

	id := resourceID(ptr)

	producer, ok := d.producers[id]
	if ok {
		producer.inuse++
		producer.stale = false

		return true
	}

	return false
}

func (d *DedupScheduler) unref(ptr resource.Pointer) {
	d.producersMu.Lock()
	defer d.producersMu.Unlock()

	id := resourceID(ptr)

	producer, ok := d.producers[id]
	if ok {
		producer.inuse--
	}
}

// Start the producer by id.
func (d *DedupScheduler) Start(ctx context.Context, ptr resource.Pointer) error {
	id := resourceID(ptr)

	if d.ref(ptr) {
		return nil
	}

	ch := d.sf.DoChan(id, func() (any, error) {
		d.logger.Debug("starting producer", zap.String("type", ptr.Type()), zap.String("id", ptr.ID()))

		p, err := d.factory(ctx, d.state, ptr, d.logger)
		if err != nil {
			return nil, err
		}

		if err = p.Start(); err != nil {
			return nil, err
		}

		d.producersMu.Lock()
		defer d.producersMu.Unlock()

		d.producers[id] = &producerWrapper{
			producer: p,
			resource: ptr,
		}

		return nil, nil //nolint:nilnil
	})

	select {
	case <-ctx.Done():
		return ctx.Err()
	case v := <-ch:
		if v.Err != nil {
			return v.Err
		}

		d.ref(ptr)

		return nil
	}
}

// Stop the producer by id.
func (d *DedupScheduler) Stop(ptr resource.Pointer) {
	d.unref(ptr)
}

// Describe implements prom.Collector interface.
func (d *DedupScheduler) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(d, ch)
}

// Collect implements prom.Collector interface.
func (d *DedupScheduler) Collect(ch chan<- prometheus.Metric) {
	d.producersMu.Lock()
	defer d.producersMu.Unlock()

	d.activeProducers.With(prometheus.Labels{"type": d.resourceType}).Set(float64(len(d.producers)))
	d.activeProducers.Collect(ch)
}

func resourceID(ptr resource.Pointer) string {
	return fmt.Sprintf("%s/%s", ptr.Namespace(), ptr.ID())
}

var _ prometheus.Collector = &DedupScheduler{}
