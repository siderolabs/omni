// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package ratelimit_test

import (
	"net/netip"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/siderolabs/siderolink/pkg/tun"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"golang.org/x/net/ipv6"

	"github.com/siderolabs/omni/internal/pkg/siderolink/ratelimit"
)

func TestNewLimiterNil(t *testing.T) {
	t.Parallel()

	assert.Nil(t, ratelimit.NewLimiter(0, 0, zaptest.NewLogger(t)))
}

func TestNewLimiterNonNil(t *testing.T) {
	t.Parallel()

	assert.NotNil(t, ratelimit.NewLimiter(10, 0, zaptest.NewLogger(t)))
}

func TestLimiterMetrics(t *testing.T) {
	t.Parallel()

	// 1 Mbps with small burst so we can trigger drops easily.
	limiter := ratelimit.NewLimiter(1, 1500, zaptest.NewLogger(t))
	require.NotNil(t, limiter)

	mock := &mockBind{}
	wrapped := limiter.WrapBind(mock)

	ep := &mockEndpoint{dst: "[fd00::1]:51820"}

	// First packet passes (within burst).
	err := wrapped.Send([][]byte{make([]byte, 1200)}, ep)
	require.NoError(t, err)

	// Second packet is dropped (burst exceeded).
	err = wrapped.Send([][]byte{make([]byte, 1200)}, ep)
	require.NoError(t, err)

	assert.Len(t, mock.sentBufs, 1, "only first packet should have been sent")

	// Verify Prometheus metrics reflect the drop.
	assert.InDelta(t, 1, testutil.ToFloat64(limiter.DroppedPackets().WithLabelValues(ratelimit.MetricLabelDirOutbound)), 0.01)
}

func TestInputPacketFilter(t *testing.T) {
	t.Parallel()

	// 1 Mbps, burst = 1500 bytes (enough for one ~1200 byte packet).
	limiter := ratelimit.NewLimiter(1, 1500, zaptest.NewLogger(t))
	require.NotNil(t, limiter)

	filter := limiter.InputPacketFilter()

	// PayloadLength such that ipv6.HeaderLen + PayloadLength = ~1200 bytes.
	payloadLen := uint16(1200 - ipv6.HeaderLen)

	header := tun.PacketHeader{
		SourceAddr:    netip.MustParseAddr("fd00::1"),
		PayloadLength: payloadLen,
	}

	// First packet should be allowed (filter returns false = don't drop).
	assert.False(t, filter(header), "first packet should be allowed")

	// Second packet should be dropped (filter returns true = drop).
	assert.True(t, filter(header), "second packet should be dropped after burst exceeded")

	// Verify inbound metrics.
	assert.InDelta(t, 1, testutil.ToFloat64(limiter.DroppedPackets().WithLabelValues(ratelimit.MetricLabelDirInbound)), 0.01)
}

func TestSharedBudgetAcrossPeers(t *testing.T) {
	t.Parallel()

	// 1 Mbps, burst = 2000 bytes — enough for one ~1200 byte packet but not two.
	limiter := ratelimit.NewLimiter(1, 2000, zaptest.NewLogger(t))
	require.NotNil(t, limiter)

	filter := limiter.InputPacketFilter()
	payloadLen := uint16(1200 - ipv6.HeaderLen)

	h1 := tun.PacketHeader{SourceAddr: netip.MustParseAddr("fd00::1"), PayloadLength: payloadLen}
	h2 := tun.PacketHeader{SourceAddr: netip.MustParseAddr("fd00::2"), PayloadLength: payloadLen}

	// First peer's packet passes.
	assert.False(t, filter(h1), "first packet should be allowed")

	// Second peer's packet dropped — shared budget exhausted.
	assert.True(t, filter(h2), "second packet should be dropped")
}
