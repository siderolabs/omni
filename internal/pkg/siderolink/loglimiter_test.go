// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package siderolink_test

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"
	"go.uber.org/zap/zaptest/observer"

	"github.com/siderolabs/omni/internal/pkg/siderolink"
)

// testNow is a fixed reference instant used by the limiter tests.
var testNow = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func TestNewLogIngestionLimiter_DisabledWhenRateZero(t *testing.T) {
	require.Nil(t, siderolink.NewLogIngestionLimiter(0, 0, zaptest.NewLogger(t)))
	require.Nil(t, siderolink.NewLogIngestionLimiter(0, 1000, zaptest.NewLogger(t)))
}

func TestLogIngestionLimiter_AllowsWithinBurst(t *testing.T) {
	limiter := siderolink.NewLogIngestionLimiter(1000, 100, zaptest.NewLogger(t))

	assert.True(t, limiter.Allow(testNow, "m1", 50))
	assert.True(t, limiter.Allow(testNow, "m1", 50))
}

func TestLogIngestionLimiter_RejectsAfterBurstExhausted(t *testing.T) {
	limiter := siderolink.NewLogIngestionLimiter(1000, 100, zaptest.NewLogger(t))

	require.True(t, limiter.Allow(testNow, "m1", 100))
	assert.False(t, limiter.Allow(testNow, "m1", 1))
}

func TestLogIngestionLimiter_RejectsOversizeMessages(t *testing.T) {
	limiter := siderolink.NewLogIngestionLimiter(1000, 100, zaptest.NewLogger(t))

	// size > burst can never fit, drop immediately.
	assert.False(t, limiter.Allow(testNow, "m1", 101))
	assert.False(t, limiter.Allow(testNow, "m1", 10000))
}

func TestLogIngestionLimiter_TokensRefillOverTime(t *testing.T) {
	limiter := siderolink.NewLogIngestionLimiter(1000, 100, zaptest.NewLogger(t))

	require.True(t, limiter.Allow(testNow, "m1", 100))
	require.False(t, limiter.Allow(testNow, "m1", 50))

	assert.True(t, limiter.Allow(testNow.Add(100*time.Millisecond), "m1", 50))
}

func TestLogIngestionLimiter_ForgetResetsBucket(t *testing.T) {
	limiter := siderolink.NewLogIngestionLimiter(1000, 100, zaptest.NewLogger(t))

	require.True(t, limiter.Allow(testNow, "m1", 100))
	require.False(t, limiter.Allow(testNow, "m1", 1))

	limiter.Forget("m1")

	// Next Allow gets a fresh full bucket.
	assert.True(t, limiter.Allow(testNow, "m1", 100))
}

func TestLogIngestionLimiter_PerMachineIsolation(t *testing.T) {
	limiter := siderolink.NewLogIngestionLimiter(1000, 100, zaptest.NewLogger(t))

	require.True(t, limiter.Allow(testNow, "m1", 100))
	require.False(t, limiter.Allow(testNow, "m1", 1))

	// m2 has its own bucket.
	assert.True(t, limiter.Allow(testNow, "m2", 100))
}

func TestLogIngestionLimiter_WarnIsDebounced(t *testing.T) {
	core, observed := observer.New(zapcore.WarnLevel)

	limiter := siderolink.NewLogIngestionLimiter(1000, 100, zap.New(core))

	require.True(t, limiter.Allow(testNow, "m1", 100))

	// 100 rejected calls within a minute should emit exactly one warn.
	for range 100 {
		assert.False(t, limiter.Allow(testNow, "m1", 1))
	}

	assert.Equal(t, 1, observed.FilterMessage("log ingestion rate limit exceeded, dropping log messages").Len())
}

func TestLogIngestionLimiter_WarnFiresAgainAfterDebounce(t *testing.T) {
	core, observed := observer.New(zapcore.WarnLevel)

	limiter := siderolink.NewLogIngestionLimiter(1000, 100, zap.New(core))

	require.True(t, limiter.Allow(testNow, "m1", 100))
	require.False(t, limiter.Allow(testNow, "m1", 1))

	later := testNow.Add(61 * time.Second)
	require.True(t, limiter.Allow(later, "m1", 100))
	require.False(t, limiter.Allow(later, "m1", 1))

	assert.Equal(t, 2, observed.FilterMessage("log ingestion rate limit exceeded, dropping log messages").Len())
}

func TestLogIngestionLimiter_OversizeUsesDistinctWarnAndLabel(t *testing.T) {
	core, observed := observer.New(zapcore.WarnLevel)

	limiter := siderolink.NewLogIngestionLimiter(1000, 100, zap.New(core))

	assert.False(t, limiter.Allow(testNow, "m1", 200))

	assert.Equal(t, 1, observed.FilterMessage("log ingestion message exceeds burst limit, dropping").Len())
}

func TestLogIngestionLimiter_MetricsByReason(t *testing.T) {
	registry := prometheus.NewRegistry()

	limiter := siderolink.NewLogIngestionLimiter(1000, 100, zaptest.NewLogger(t))
	registry.MustRegister(limiter)

	require.True(t, limiter.Allow(testNow, "m1", 100))
	// Oversize drop: should bump reason="oversize" by 200.
	require.False(t, limiter.Allow(testNow, "m1", 200))
	// Rate-limit drop: should bump reason="rate_limit" by 1.
	require.False(t, limiter.Allow(testNow, "m1", 1))

	counts := gatherDroppedTotals(t, registry)
	assert.Equal(t, float64(200), counts["oversize"])
	assert.Equal(t, float64(1), counts["rate_limit"])
}

func gatherDroppedTotals(t *testing.T, registry *prometheus.Registry) map[string]float64 {
	t.Helper()

	mfs, err := registry.Gather()
	require.NoError(t, err)

	out := map[string]float64{}

	for _, mf := range mfs {
		if mf.GetName() != "omni_machine_logs_dropped_total" {
			continue
		}

		for _, m := range mf.GetMetric() {
			var reason string

			for _, l := range m.GetLabel() {
				if l.GetName() == "reason" {
					reason = l.GetValue()
				}
			}

			out[reason] = m.GetCounter().GetValue()
		}
	}

	return out
}
