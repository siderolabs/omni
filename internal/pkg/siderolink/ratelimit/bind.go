// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package ratelimit

import (
	"time"

	"golang.zx2c4.com/wireguard/conn"
)

// rateLimitedBind wraps a conn.Bind to drop outbound packets that exceed
// the bandwidth budget.
type rateLimitedBind struct {
	conn.Bind
	limiter *Limiter
}

// Send implements conn.Bind. It drops packets that exceed the rate limit.
func (b *rateLimitedBind) Send(bufs [][]byte, ep conn.Endpoint) error {
	now := time.Now()

	filtered := make([][]byte, 0, len(bufs))

	for _, buf := range bufs {
		if b.limiter.allow(now, len(buf), MetricLabelDirOutbound) {
			filtered = append(filtered, buf)
		}
	}

	if len(filtered) == 0 {
		return nil
	}

	return b.Bind.Send(filtered, ep)
}
