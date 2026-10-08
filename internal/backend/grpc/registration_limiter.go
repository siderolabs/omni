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
	registrationBurst         = 100
	registrationInterval      = 6 * time.Second // 10 per minute
	registrationSweepInterval = time.Minute
	forwardedForMetadataKey   = "x-forwarded-for"
	ipv6SourcePrefixLength    = 64
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

	if now.Sub(l.lastSweep) > registrationSweepInterval {
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

// sourceAddress returns the first X-Forwarded-For entry, which is the client address set by setClientAddress (the
// REST gateway appends the connection address after it).
//
// An IPv6 address is reduced to its /64 prefix. Unlike with IPv4, a network usually gets a whole /64, and every host
// in it can use any address from it, so limiting single addresses would be easy to get around.
func sourceAddress(ctx context.Context) string {
	md, _ := metadata.FromIncomingContext(ctx)

	values := md.Get(forwardedForMetadataKey)
	if len(values) == 0 {
		return ""
	}

	first, _, _ := strings.Cut(values[0], ",")
	addr, _ := netip.ParseAddr(strings.TrimSpace(first)) //nolint:errcheck // set by setClientAddress
	addr = addr.Unmap()

	if addr.Is6() {
		return netip.PrefixFrom(addr, ipv6SourcePrefixLength).Masked().String()
	}

	return addr.String()
}
