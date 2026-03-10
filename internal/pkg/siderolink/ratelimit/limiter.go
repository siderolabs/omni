// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package ratelimit provides bandwidth rate limiting for the SideroLink WireGuard tunnel.
//
// Rate limiting is applied at two points:
//   - Outbound: WireGuard Bind Send (packets from Omni to peers) via WrapBind
//   - Inbound: TUN device Write (packets from peers to Omni) via InputPacketFilter
//
// A single token bucket is shared across all peers and both directions.
// When the limit is exceeded, packets are dropped.
// TCP handles this naturally via congestion control.
//
// Note: inbound rate limiting drops packets after they have already traversed the wire.
// It is still useful because TCP congestion control on the sender will back off,
// reducing sustained inbound throughput to the configured limit.
package ratelimit

import (
	"math"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/siderolabs/siderolink/pkg/tun"
	"go.uber.org/zap"
	"golang.org/x/net/ipv6"
	"golang.org/x/time/rate"
	"golang.zx2c4.com/wireguard/conn"
)

const (
	MetricLabelDirInbound  = "inbound"
	MetricLabelDirOutbound = "outbound"
)

const logDebounceInterval = time.Minute

// Limiter applies bandwidth rate limiting using a single shared token bucket.
type Limiter struct {
	limiter        *rate.Limiter
	droppedPackets *prometheus.CounterVec
	logger         *zap.Logger
	lastLogTime    atomic.Int64
}

var _ prometheus.Collector = &Limiter{}

// NewLimiter creates a Limiter with the given rate limit.
// Returns nil if mbps is 0 (unlimited).
// When burstBytes is 0, burst defaults to one second worth of the rate.
func NewLimiter(mbps, burstBytes uint64, logger *zap.Logger) *Limiter {
	if mbps == 0 {
		return nil
	}

	bytesPerSec := mbpsToByteRate(mbps)
	burst := clampToInt(burstBytes)

	if burst == 0 {
		burst = clampToInt(bytesPerSec)
	}

	return &Limiter{
		limiter: rate.NewLimiter(rate.Limit(bytesPerSec), burst),
		droppedPackets: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "omni_siderolink_ratelimit_dropped_packets_total",
			Help: "Total packets dropped by SideroLink bandwidth rate limiting.",
		}, []string{"direction"}),
		logger: logger,
	}
}

// InputPacketFilter returns a tun.InputPacketFilter that rate-limits inbound packets.
// Packets that exceed the bandwidth budget are dropped.
func (l *Limiter) InputPacketFilter() tun.InputPacketFilter {
	return func(header tun.PacketHeader) bool {
		packetSize := ipv6.HeaderLen + int(header.PayloadLength)

		return !l.allow(time.Now(), packetSize, MetricLabelDirInbound)
	}
}

// WrapBind wraps a conn.Bind to rate-limit outbound packets in Send.
func (l *Limiter) WrapBind(bind conn.Bind) conn.Bind {
	return &rateLimitedBind{Bind: bind, limiter: l}
}

// allow checks the shared rate limit and increments the drop metric on failure.
func (l *Limiter) allow(now time.Time, n int, direction string) bool {
	if !l.limiter.AllowN(now, n) {
		l.droppedPackets.WithLabelValues(direction).Inc()

		if lastLog := l.lastLogTime.Load(); now.UnixNano()-lastLog >= int64(logDebounceInterval) {
			if l.lastLogTime.CompareAndSwap(lastLog, now.UnixNano()) {
				l.logger.Warn("SideroLink bandwidth rate limit exceeded, dropping packets", zap.String("direction", direction))
			}
		}

		return false
	}

	return true
}

// DroppedPackets returns the dropped packets counter vec (for testing).
func (l *Limiter) DroppedPackets() *prometheus.CounterVec { return l.droppedPackets }

// Describe implements prometheus.Collector.
func (l *Limiter) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(l, ch)
}

// Collect implements prometheus.Collector.
func (l *Limiter) Collect(ch chan<- prometheus.Metric) {
	l.droppedPackets.Collect(ch)
}

// mbpsToByteRate converts megabits-per-second to bytes-per-second, clamping to
// prevent uint64 overflow on the intermediate multiplication.
func mbpsToByteRate(mbps uint64) uint64 {
	const maxMbps = math.MaxUint64 / 1_000_000

	return min(mbps, maxMbps) * 1_000_000 / 8
}

// clampToInt safely converts a uint64 to int, clamping at math.MaxInt.
func clampToInt(v uint64) int {
	if v > uint64(math.MaxInt) {
		return math.MaxInt
	}

	return int(v)
}
