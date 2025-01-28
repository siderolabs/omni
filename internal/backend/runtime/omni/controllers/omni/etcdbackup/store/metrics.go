// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package store

import (
	"context"
	"io"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/etcdbackup"
)

// FactoryWithMetrics is a factory for etcd backup stores with metrics.
type FactoryWithMetrics interface {
	Factory
	prometheus.Collector
}

type factoryWithMetrics struct {
	factory       Factory
	uploads       *prometheus.CounterVec
	downloads     *prometheus.CounterVec
	uploadBytes   prometheus.Counter
	downloadBytes prometheus.Counter
}

func newFactoryWithMetrics(factory Factory) FactoryWithMetrics {
	return &factoryWithMetrics{
		factory: factory,
		uploads: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "omni_etcdbackup_store_uploads_total",
			Help: "Number of etcd backup uploads",
		}, []string{"status"}),
		downloads: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "omni_etcdbackup_store_downloads_total",
			Help: "Number of etcd backup downloads",
		}, []string{"status"}),
		uploadBytes: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "omni_etcdbackup_store_transferred_bytes_total",
			Help: "Number of bytes transferred during etcd backup uploads",
		}),
		downloadBytes: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "omni_etcdbackup_store_downloaded_bytes_total",
			Help: "Number of bytes transferred during etcd backup downloads",
		}),
	}
}

func (s *factoryWithMetrics) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(s, ch)
}

func (s *factoryWithMetrics) Collect(ch chan<- prometheus.Metric) {
	s.uploads.Collect(ch)
	s.downloads.Collect(ch)
	s.uploadBytes.Collect(ch)
	s.downloadBytes.Collect(ch)
}

func (s *factoryWithMetrics) Start(ctx context.Context, state state.State, logger *zap.Logger) error {
	return s.factory.Start(ctx, state, logger)
}

func (s *factoryWithMetrics) GetStore() (etcdbackup.Store, error) {
	store, err := s.factory.GetStore()
	if err != nil {
		return nil, err
	}

	return &storeWithMetrics{
		store:   store,
		metrics: s,
	}, nil
}

func (s *factoryWithMetrics) Description() string {
	return s.factory.Description()
}

type storeWithMetrics struct {
	store   etcdbackup.Store
	metrics *factoryWithMetrics
}

func (s *storeWithMetrics) ListBackups(ctx context.Context, clusterUUID string) (etcdbackup.InfoIterator, error) {
	return s.store.ListBackups(ctx, clusterUUID)
}

func (s *storeWithMetrics) Upload(ctx context.Context, descr etcdbackup.Description, r io.Reader) error {
	err := s.store.Upload(ctx, descr, &readCloserWithMetrics{
		readCloser: io.NopCloser(r),
		counter:    s.metrics.uploadBytes,
	})
	if err != nil {
		s.metrics.uploads.WithLabelValues("error").Inc()

		return err
	}

	s.metrics.uploads.WithLabelValues("success").Inc()

	return nil
}

func (s *storeWithMetrics) Download(ctx context.Context, encryptionKey []byte, clusterUUID, snapshotName string) (etcdbackup.BackupData, io.ReadCloser, error) {
	backupData, readCloser, err := s.store.Download(ctx, encryptionKey, clusterUUID, snapshotName)
	if err != nil {
		s.metrics.downloads.WithLabelValues("error").Inc()

		return backupData, readCloser, err
	}

	s.metrics.downloads.WithLabelValues("success").Inc()

	metricsWrapper := readCloserWithMetrics{
		readCloser: readCloser,
		counter:    s.metrics.downloadBytes,
	}

	return backupData, &metricsWrapper, err
}

type readCloserWithMetrics struct {
	readCloser io.ReadCloser
	counter    prometheus.Counter
}

func (r *readCloserWithMetrics) Read(p []byte) (n int, err error) {
	n, err = r.readCloser.Read(p)

	r.counter.Add(float64(n))

	return n, err
}

func (r *readCloserWithMetrics) Close() error {
	return r.readCloser.Close()
}
