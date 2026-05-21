// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package ratelimit_test

import (
	"context"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/omni/internal/backend/ratelimit"
	"github.com/siderolabs/omni/internal/pkg/config"
)

func throttleConfig(perSec, burst uint64, maxWait time.Duration) config.EtcdWriteRateLimits {
	cls := config.EtcdClassWriteRateLimit{}
	cls.SetWriteBytesPerSecond(perSec)
	cls.SetWriteBytesBurst(burst)

	cfg := config.EtcdWriteRateLimits{
		User:          cls,
		InfraProvider: cls,
		Internal:      cls,
	}
	cfg.SetMaxWait(maxWait)

	return cfg
}

func TestThrottlerNilIsPassthrough(t *testing.T) {
	t.Parallel()

	var thr *ratelimit.Throttler

	assert.NoError(t, thr.Gate(t.Context(), 1024, "T"))
	assert.Nil(t, thr.LimiterFunc())
}

func TestThrottlerZeroConfigShortCircuits(t *testing.T) {
	t.Parallel()

	thr := ratelimit.New(config.EtcdWriteRateLimits{}, nil)
	assert.Nil(t, thr)
	assert.Nil(t, thr.LimiterFunc())

	for range 10_000 {
		require.NoError(t, thr.Gate(t.Context(), 1024, "T"))
	}
}

func TestThrottlerAdmitsUnderBudget(t *testing.T) {
	t.Parallel()

	thr := ratelimit.New(throttleConfig(1<<20, 1<<20, time.Second), nil)

	require.NoError(t, thr.Gate(t.Context(), 1024, "T"))
}

func TestThrottlerBlocksThenAdmits(t *testing.T) {
	t.Parallel()

	thr := ratelimit.New(throttleConfig(1024, 1024, 5*time.Second), nil)

	ctx := t.Context()

	require.NoError(t, thr.Gate(ctx, 1024, "T"))

	start := time.Now()

	require.NoError(t, thr.Gate(ctx, 1024, "T"))

	elapsed := time.Since(start)

	assert.GreaterOrEqual(t, elapsed, 500*time.Millisecond, "second admission must have waited for refill")
}

func TestThrottlerMaxWaitTimesOut(t *testing.T) {
	t.Parallel()

	thr := ratelimit.New(throttleConfig(1, 1024, 100*time.Millisecond), nil)

	require.NoError(t, thr.Gate(t.Context(), 1024, "T"))

	start := time.Now()
	err := thr.Gate(t.Context(), 1024, "T")
	elapsed := time.Since(start)

	require.Error(t, err)
	assert.Equal(t, codes.DeadlineExceeded, status.Code(err))
	assert.LessOrEqual(t, elapsed, time.Second, "should not block longer than ~maxWait")
}

func TestThrottlerContextDeadlineTakesPrecedenceOverMaxWait(t *testing.T) {
	t.Parallel()

	thr := ratelimit.New(throttleConfig(1, 1024, 10*time.Second), nil)

	require.NoError(t, thr.Gate(t.Context(), 1024, "T"))

	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := thr.Gate(ctx, 1024, "T")
	elapsed := time.Since(start)

	require.Error(t, err)
	assert.Equal(t, codes.DeadlineExceeded, status.Code(err))
	assert.LessOrEqual(t, elapsed, time.Second, "should not block longer than ~ctx deadline")
}

func TestThrottlerPayloadLargerThanBurst(t *testing.T) {
	t.Parallel()

	reg := prometheus.NewRegistry()
	thr := ratelimit.New(throttleConfig(1024, 1024, 10*time.Second), reg)

	start := time.Now()
	err := thr.Gate(t.Context(), 2048, "T")
	elapsed := time.Since(start)

	require.Error(t, err)
	assert.Equal(t, codes.ResourceExhausted, status.Code(err))
	assert.Less(t, elapsed, 100*time.Millisecond, "must return immediately, no waiting")

	// Oversize rejections must be distinguishable from timeouts in metrics.
	oversize := readCounter(t, reg, "omni_etcd_write_throttle_failed_total", map[string]string{"class": "user", "reason": "oversize"})
	assert.InDelta(t, 1.0, oversize, 0.0001)

	timeouts := readCounter(t, reg, "omni_etcd_write_throttle_failed_total", map[string]string{"class": "user", "reason": "timeout"})
	assert.InDelta(t, 0.0, timeouts, 0.0001, "oversize must NOT show up under reason=timeout")
}

func TestThrottlerFailedMetricsRecordPayloadSize(t *testing.T) {
	t.Parallel()

	reg := prometheus.NewRegistry()
	thr := ratelimit.New(throttleConfig(1, 64, 50*time.Millisecond), reg)

	require.NoError(t, thr.Gate(t.Context(), 64, "T"))

	err := thr.Gate(t.Context(), 64, "T")
	require.Error(t, err)
	assert.Equal(t, codes.DeadlineExceeded, status.Code(err))

	failedBytes := readCounter(t, reg, "omni_etcd_write_throttle_failed_bytes_total", map[string]string{"class": "user", "reason": "timeout"})
	assert.InDelta(t, 64.0, failedBytes, 0.0001, "failed_bytes must record payload size, not event count")

	failedEvents := readCounter(t, reg, "omni_etcd_write_throttle_failed_total", map[string]string{"class": "user", "reason": "timeout"})
	assert.InDelta(t, 1.0, failedEvents, 0.0001)

	admitted := readCounter(t, reg, "omni_etcd_write_throttle_admitted_total", map[string]string{"class": "user"})
	assert.InDelta(t, 1.0, admitted, 0.0001)
}

func TestThrottlerHistogramObservesAdmittedWaits(t *testing.T) {
	t.Parallel()

	reg := prometheus.NewRegistry()
	thr := ratelimit.New(throttleConfig(1000, 1000, time.Second), reg)

	require.NoError(t, thr.Gate(t.Context(), 1000, "T"))

	start := time.Now()

	require.NoError(t, thr.Gate(t.Context(), 500, "T"))

	elapsed := time.Since(start)

	assert.GreaterOrEqual(t, elapsed, 400*time.Millisecond, "real wait should have happened")

	hist := readHistogram(t, reg, "omni_etcd_write_throttle_wait_seconds", map[string]string{"class": "user"})
	assert.Equal(t, uint64(2), hist.GetSampleCount())
	assert.GreaterOrEqual(t, hist.GetSampleSum(), 0.4, "histogram must include the ~500ms wait of the second admission")
}

func TestThrottlerAdmittedCounterIncrements(t *testing.T) {
	t.Parallel()

	reg := prometheus.NewRegistry()
	thr := ratelimit.New(throttleConfig(10_000, 10_000, time.Second), reg)

	for range 5 {
		require.NoError(t, thr.Gate(t.Context(), 100, "T"))
	}

	admitted := readCounter(t, reg, "omni_etcd_write_throttle_admitted_total", map[string]string{"class": "user"})
	assert.InDelta(t, 5.0, admitted, 0.0001)
}

func TestThrottlerLimiterFuncDelegatesToGate(t *testing.T) {
	t.Parallel()

	thr := ratelimit.New(throttleConfig(64, 64, 50*time.Millisecond), nil)

	gate := thr.LimiterFunc()
	require.NotNil(t, gate)

	// First call within burst admits.
	require.NoError(t, gate(t.Context(), state.Created, "T", resource.PhaseRunning, resource.PhaseRunning, 50))

	// Second call would need to wait > 50ms; maxWait fires.
	err := gate(t.Context(), state.Created, "T", resource.PhaseRunning, resource.PhaseRunning, 50)
	require.Error(t, err)
	assert.Equal(t, codes.DeadlineExceeded, status.Code(err))
}

func readCounter(t *testing.T, reg *prometheus.Registry, name string, labels map[string]string) float64 {
	t.Helper()

	families, err := reg.Gather()
	require.NoError(t, err)

	for _, f := range families {
		if f.GetName() != name {
			continue
		}

		for _, m := range f.GetMetric() {
			if labelsMatch(m.GetLabel(), labels) {
				return m.GetCounter().GetValue()
			}
		}
	}

	t.Fatalf("counter %s with labels %v not found", name, labels)

	return 0
}

func readHistogram(t *testing.T, reg *prometheus.Registry, name string, labels map[string]string) *dto.Histogram {
	t.Helper()

	families, err := reg.Gather()
	require.NoError(t, err)

	for _, f := range families {
		if f.GetName() != name {
			continue
		}

		for _, m := range f.GetMetric() {
			if labelsMatch(m.GetLabel(), labels) {
				return m.GetHistogram()
			}
		}
	}

	t.Fatalf("histogram %s with labels %v not found", name, labels)

	return nil
}

func labelsMatch(have []*dto.LabelPair, want map[string]string) bool {
	if len(have) < len(want) {
		return false
	}

	for k, v := range want {
		found := false

		for _, lp := range have {
			if lp.GetName() == k && lp.GetValue() == v {
				found = true

				break
			}
		}

		if !found {
			return false
		}
	}

	return true
}
