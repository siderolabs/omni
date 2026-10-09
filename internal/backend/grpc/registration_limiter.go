// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package grpc

import (
	"context"
	"net/netip"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
	"google.golang.org/grpc/metadata"
)

const (
	registrationBurst        = 100
	registrationInterval     = 6 * time.Second // 10 per minute
	registrationLimiterSweep = time.Minute
	forwardedForMetadataKey  = "x-forwarded-for"
	ipv6SourceBits           = 64
)

// registrationLimiter limits the public key registrations per source address.
type registrationLimiter struct {
	sources   map[string]*rate.Limiter
	lastSweep time.Time
	mu        sync.Mutex
}

func (l *registrationLimiter) allow(source string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.sources == nil {
		l.sources = map[string]*rate.Limiter{}
	}

	if now.Sub(l.lastSweep) > registrationLimiterSweep {
		for addr, limiter := range l.sources {
			if limiter.TokensAt(now) >= registrationBurst { // same as a new one
				delete(l.sources, addr)
			}
		}

		l.lastSweep = now
	}

	limiter, ok := l.sources[source]
	if !ok {
		limiter = rate.NewLimiter(rate.Every(registrationInterval), registrationBurst)
		l.sources[source] = limiter
	}

	return limiter.AllowN(now, 1)
}

// sourceAddress is the address of the client, an IPv6 address by its /64 prefix.
//
// The API server decides the client address at the edge and sets it as the first X-Forwarded-For entry, see
// setClientAddress. The REST gateway appends the connection address after it, so only the first entry counts.
func sourceAddress(ctx context.Context) string {
	md, _ := metadata.FromIncomingContext(ctx)

	values := md.Get(forwardedForMetadataKey)
	if len(values) == 0 {
		return ""
	}

	first, _, _ := strings.Cut(values[0], ",")
	addr, _ := netip.ParseAddr(strings.TrimSpace(first)) //nolint:errcheck // the edge writes a parsed address
	addr = addr.Unmap()

	if addr.Is6() {
		return netip.PrefixFrom(addr, ipv6SourceBits).Masked().String()
	}

	return addr.String()
}
