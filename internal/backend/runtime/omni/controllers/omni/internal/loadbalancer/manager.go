// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package loadbalancer

import (
	"errors"
	"fmt"

	"github.com/siderolabs/gen/maps"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/xslices"
	"go.uber.org/zap"
)

// ID is a loadbalancer ID.
type ID = string

// Manager manages running loadbalancers.
type Manager struct {
	running map[ID]wrapper
	logger  *zap.Logger
	newFunc NewFunc
}

// Spec configures a loadbalancer.
type Spec struct {
	BindAddress string
	BindPort    int
}

// wrapper wraps a loadbalancer and its upstream channel.
type wrapper struct {
	lb          LoadBalancer
	upstreamCh  chan []string
	spec        Spec
	lastHealthy optional.Optional[bool]
}

// NewManager returns a new loadbalancer manager.
func NewManager(logger *zap.Logger, newFunc NewFunc) *Manager {
	return &Manager{
		running: make(map[ID]wrapper),
		logger:  logger,
		newFunc: newFunc,
	}
}

// Stop stops all running loadbalancers.
func (m *Manager) Stop() error {
	return errors.Join(xslices.Map(maps.Values(m.running), func(w wrapper) error {
		return w.lb.Shutdown()
	})...)
}

// Reconcile by starting/stopping/replacing loadbalancer as needed.
func (m *Manager) Reconcile(specs map[ID]Spec) error {
	var allErrors []error

	// stop all loadbalancers that should not be running
	for id, w := range m.running {
		if newSpec, ok := specs[id]; !ok || newSpec != w.spec {
			if !ok {
				m.logger.Debug("stopping loadbalancer", zap.String("id", id), zap.Int("port", w.spec.BindPort))
			} else {
				m.logger.Debug("replacing loadbalancer", zap.String("id", id), zap.Int("port", w.spec.BindPort))
			}

			if err := w.lb.Shutdown(); err != nil {
				allErrors = append(allErrors, fmt.Errorf("error stopping %s: %w", id, err))
			}

			delete(m.running, id)
		}
	}

	// start all loadbalancers that should be running
	for id, spec := range specs {
		if _, running := m.running[id]; !running {
			m.logger.Debug("starting loadbalancer", zap.String("id", id), zap.Int("port", spec.BindPort))

			w := wrapper{
				spec:       spec,
				upstreamCh: make(chan []string),
			}

			lb, err := m.newFunc(spec.BindAddress, spec.BindPort, m.logger)
			if err != nil {
				allErrors = append(allErrors, fmt.Errorf("error creating loadbalancer %s: %w", id, err))

				continue
			}

			if err = lb.Start(w.upstreamCh); err != nil {
				allErrors = append(allErrors, fmt.Errorf("error starting loadbalancer %s: %w", id, err))

				continue
			}

			w.lb = lb
			m.running[id] = w
		}
	}

	return errors.Join(allErrors...)
}

// GetHealthStatus compiles the health status of all loadbalancers, returning the diff with previous reported status.
func (m *Manager) GetHealthStatus() map[ID]bool {
	status := map[ID]bool{}

	for id, w := range m.running {
		healthy, err := w.lb.Healthy()
		if err != nil {
			m.logger.Error("error getting loadbalancer health", zap.String("id", id), zap.Error(err))

			healthy = false
		}

		previousHealthy, exists := w.lastHealthy.Get()
		if !exists || healthy != previousHealthy {
			status[id] = healthy
		}

		w.lastHealthy = optional.Some(healthy)
	}

	return status
}

// UpdateUpstreams updates the upstreams of all loadbalancers.
func (m *Manager) UpdateUpstreams(upstreams map[ID][]string) {
	// running loadbalancers always consume from the channel
	// also loadbalancers ignore if nothing changed, so we can always send
	for id, w := range m.running {
		w.upstreamCh <- upstreams[id]
	}
}
