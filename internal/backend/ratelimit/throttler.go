// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package ratelimit

import (
	"context"
	"math"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/time/rate"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/omni/internal/pkg/auth/actor"
	"github.com/siderolabs/omni/internal/pkg/config"
)

const (
	reasonTimeout  = "timeout"
	reasonOversize = "oversize"
)

// Throttler gates etcd mutations by marshaled-payload bytes/sec, per caller class.
//
// A nil *Throttler is a valid passthrough: Gate returns nil and LimiterFunc returns nil (making state-etcd's WithLimiter option a no-op).
type Throttler struct {
	buckets             [bucketCount]*rate.Limiter
	waitSeconds         [bucketCount]prometheus.Observer
	admitted            [bucketCount]prometheus.Counter
	failedTimeout       [bucketCount]prometheus.Counter
	failedOversize      [bucketCount]prometheus.Counter
	failedTimeoutBytes  [bucketCount]prometheus.Counter
	failedOversizeBytes [bucketCount]prometheus.Counter
	maxWait             time.Duration
}

// New constructs a Throttler from storage.rateLimits.etcd. Each class's bucket is independent; setting writeBytesPerSecond=0 or writeBytesBurst=0 disables the throttle for that class.
//
// Returns nil when every class is disabled, so state-etcd's WithLimiter becomes a true no-op (no metric registration, no per-mutation overhead).
func New(cfg config.EtcdWriteRateLimits, registry prometheus.Registerer) *Throttler {
	var buckets [bucketCount]*rate.Limiter

	classCfg := [bucketCount]config.EtcdClassWriteRateLimit{
		bucketInternal:      cfg.Internal,
		bucketInfraProvider: cfg.InfraProvider,
		bucketUser:          cfg.User,
	}

	anyEnabled := false

	for i, c := range classCfg {
		buckets[i] = buildLimiter(c.GetWriteBytesPerSecond(), c.GetWriteBytesBurst())
		if buckets[i] != nil {
			anyEnabled = true
		}
	}

	if !anyEnabled {
		return nil
	}

	waitVec := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "omni",
		Subsystem: "etcd_write_throttle",
		Name:      "wait_seconds",
		Help: "Distribution of wait time spent in the etcd write-bytes throttle, per caller class. " +
			"Successful admissions and timed-out failures are both observed (failures observe their wait-until-timeout duration).",
		Buckets: []float64{0, 0.001, 0.01, 0.1, 0.5, 1, 2.5, 5, 10, 30},
	}, []string{"class"})
	admittedVec := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "omni",
		Subsystem: "etcd_write_throttle",
		Name:      "admitted_total",
		Help:      "Number of etcd writes admitted by the throttle (after waiting if needed), per caller class.",
	}, []string{"class"})
	failedVec := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "omni",
		Subsystem: "etcd_write_throttle",
		Name:      "failed_total",
		Help: "Number of etcd writes the throttle rejected, per caller class and reason. " +
			"reason=timeout: the wait exceeded ctx deadline or maxWait. reason=oversize: payload exceeded the configured burst. " +
			"These mutations did NOT reach etcd.",
	}, []string{"class", "reason"})
	failedBytesVec := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "omni",
		Subsystem: "etcd_write_throttle",
		Name:      "failed_bytes_total",
		Help: "Total marshaled bytes of etcd writes the throttle rejected, per caller class and reason. " +
			"Comparable to omni_etcd_resource_bytes_total — this is the fraction that did NOT reach etcd.",
	}, []string{"class", "reason"})

	t := &Throttler{
		buckets: buckets,
		maxWait: cfg.GetMaxWait(),
	}

	for i := range bucketCount {
		label := bucketLabels[i]
		t.waitSeconds[i] = waitVec.WithLabelValues(label)
		t.admitted[i] = admittedVec.WithLabelValues(label)
		t.failedTimeout[i] = failedVec.WithLabelValues(label, reasonTimeout)
		t.failedOversize[i] = failedVec.WithLabelValues(label, reasonOversize)
		t.failedTimeoutBytes[i] = failedBytesVec.WithLabelValues(label, reasonTimeout)
		t.failedOversizeBytes[i] = failedBytesVec.WithLabelValues(label, reasonOversize)
	}

	if registry != nil {
		registry.MustRegister(waitVec, admittedVec, failedVec, failedBytesVec)
	}

	return t
}

// Gate blocks until n bytes of budget are available for the caller's class, ctx is canceled, or maxWait elapses — whichever comes first.
//
// Returns nil on admission, codes.DeadlineExceeded on timeout, codes.ResourceExhausted when n exceeds the configured burst (no amount of waiting would let it through).
// n <= 0 is treated as 1 (etcd still produces a revision for empty-payload deletes).
func (t *Throttler) Gate(ctx context.Context, n int, resType resource.Type) error {
	if t == nil {
		return nil
	}

	bucket := bucketFor(actor.Classify(ctx).Type)

	limiter := t.buckets[bucket]
	if limiter == nil {
		return nil
	}

	if n <= 0 {
		n = 1
	}

	// Payloads larger than the burst never admit. Surface that as a separate code so operators can distinguish "configure higher burst" from "loaded too much".
	if n > limiter.Burst() {
		t.failedOversize[bucket].Inc()
		t.failedOversizeBytes[bucket].Add(float64(n))

		return status.Errorf(codes.ResourceExhausted,
			"etcd write payload (%d bytes) exceeds the configured burst (%d) for class=%s, type=%s",
			n, limiter.Burst(), bucketLabels[bucket], resType)
	}

	if limiter.AllowN(time.Now(), n) {
		t.admitted[bucket].Inc()
		t.waitSeconds[bucket].Observe(0)

		return nil
	}

	waitCtx := ctx

	if t.maxWait > 0 {
		var cancel context.CancelFunc

		waitCtx, cancel = context.WithTimeout(ctx, t.maxWait)
		defer cancel()
	}

	start := time.Now()

	err := limiter.WaitN(waitCtx, n)

	elapsed := time.Since(start).Seconds()
	t.waitSeconds[bucket].Observe(elapsed)

	if err == nil {
		t.admitted[bucket].Inc()

		return nil
	}

	t.failedTimeout[bucket].Inc()
	t.failedTimeoutBytes[bucket].Add(float64(n))

	return status.Errorf(codes.DeadlineExceeded,
		"etcd write throttled out after %.3fs (class=%s, type=%s, bytes=%d): %v",
		elapsed, bucketLabels[bucket], resType, n, err)
}

// LimiterFunc returns a function shaped like state-etcd's etcd.LimiterFunc, suitable for wiring via etcd.WithLimiter.
func (t *Throttler) LimiterFunc() func(ctx context.Context, eventType state.EventType, resourceType resource.Type, phase, previousPhase resource.Phase, marshaledBytes int) error {
	if t == nil {
		return nil
	}

	return func(ctx context.Context, _ state.EventType, resourceType resource.Type, _, _ resource.Phase, marshaledBytes int) error {
		return t.Gate(ctx, marshaledBytes, resourceType)
	}
}

func buildLimiter(perSecond, burst uint64) *rate.Limiter {
	if perSecond == 0 || burst == 0 {
		return nil
	}

	if burst > math.MaxInt {
		burst = math.MaxInt
	}

	return rate.NewLimiter(rate.Limit(perSecond), int(burst))
}
