// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package siderolink

import (
	"math"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/siderolabs/gen/containers"
	"go.uber.org/zap"
	"golang.org/x/time/rate"

	"github.com/siderolabs/omni/internal/backend/logging"
)

const logLimiterDebounceInterval = time.Minute

// LogIngestionLimiter limits machine log ingestion rates per machine.
//
// Per-machine entries are created lazily and only removed via Forget when the omni.Machine resource is destroyed or Omni instance is restarted.
type LogIngestionLimiter struct {
	droppedLogs *prometheus.CounterVec
	logger      *zap.Logger
	limiters    containers.SyncMap[MachineID, *machineLimiter]
	rateBytes   uint64
	burstBytes  int
}

type machineLimiter struct {
	limiter     *rate.Limiter
	lastLogTime atomic.Int64
}

var _ prometheus.Collector = &LogIngestionLimiter{}

// NewLogIngestionLimiter creates a LogIngestionLimiter with the given rate limit. Returns nil if rateBytes is 0 (unlimited).
func NewLogIngestionLimiter(rateBytes, burstBytes uint64, logger *zap.Logger) *LogIngestionLimiter {
	if rateBytes == 0 {
		return nil
	}

	burst := clampToInt(burstBytes)
	if burst == 0 {
		// Default to one second worth of rate.
		burst = clampToInt(rateBytes)
	}

	return &LogIngestionLimiter{
		rateBytes:  rateBytes,
		burstBytes: burst,
		droppedLogs: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "omni_machine_logs_dropped_total",
			Help: "Total bytes of log messages dropped by log ingestion handling.",
		}, []string{"reason"}),
		logger: logger.With(logging.Component("siderolink_log_ingestion_limiter")),
	}
}

// Allow checks the rate limit for the given machine ID. If the limit is exceeded, it returns false and the message should be dropped.
func (l *LogIngestionLimiter) Allow(now time.Time, machineID MachineID, size int) bool {
	ml := l.getOrCreate(machineID)

	if ml.limiter.AllowN(now, size) {
		return true
	}

	reason := "rate_limit"
	message := "log ingestion rate limit exceeded, dropping log messages"

	if size > l.burstBytes {
		reason = "oversize"
		message = "log ingestion message exceeds burst limit, dropping"
	}

	l.droppedLogs.WithLabelValues(reason).Add(float64(size))

	if lastLog := ml.lastLogTime.Load(); now.UnixNano()-lastLog >= int64(logLimiterDebounceInterval) {
		if ml.lastLogTime.CompareAndSwap(lastLog, now.UnixNano()) {
			l.logger.Warn(message, zap.String("machine_id", string(machineID)), zap.Int("size", size))
		}
	}

	return false
}

// Forget removes the limiter entry for the given machine ID.
func (l *LogIngestionLimiter) Forget(machineID MachineID) {
	l.limiters.Delete(machineID)
}

func (l *LogIngestionLimiter) getOrCreate(machineID MachineID) *machineLimiter {
	if ml, ok := l.limiters.Load(machineID); ok {
		return ml
	}

	ml := &machineLimiter{
		limiter: rate.NewLimiter(rate.Limit(l.rateBytes), l.burstBytes),
	}

	actual, _ := l.limiters.LoadOrStore(machineID, ml)

	return actual
}

// Describe implements prometheus.Collector.
func (l *LogIngestionLimiter) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(l, ch)
}

// Collect implements prometheus.Collector.
func (l *LogIngestionLimiter) Collect(ch chan<- prometheus.Metric) {
	l.droppedLogs.Collect(ch)
}

// clampToInt safely converts a uint64 to int, clamping at math.MaxInt.
func clampToInt(v uint64) int {
	if v > uint64(math.MaxInt) {
		return math.MaxInt
	}

	return int(v)
}
